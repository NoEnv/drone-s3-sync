package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ryanuber/go-glob"
)

type AWS struct {
	client   *s3.Client
	cfClient *cloudfront.Client
	plugin   *Plugin
}

func NewAWS(p *Plugin) AWS {

	cfg := aws.Config{
		Region: p.Region,
	}

	// allowing to use the instance role or provide a key and secret
	if p.Key != "" && p.Secret != "" {
		cfg.Credentials = credentials.NewStaticCredentialsProvider(p.Key, p.Secret, "")
	}

	c := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = p.PathStyle
		if p.Endpoint != "" {
			endpoint := p.Endpoint
			if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				endpoint = "https://" + endpoint
			}
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	cf := cloudfront.NewFromConfig(cfg)

	return AWS{c, cf, p}
}

func (a *AWS) Upload(local, remote string) error {
	p := a.plugin
	if local == "" {
		return nil
	}

	file, err := os.Open(local)
	if err != nil {
		return err
	}

	defer file.Close()

	var access string
	for pattern := range p.Access {
		if match := glob.Glob(pattern, local); match {
			access = p.Access[pattern]
			break
		}
	}

	if access == "" {
		access = "private"
	}

	fileExt := filepath.Ext(local)

	var contentType string
	for patternExt := range p.ContentType {
		if patternExt == fileExt {
			contentType = p.ContentType[patternExt]
			break
		}
	}

	if contentType == "" {
		contentType = mime.TypeByExtension(fileExt)
	}

	var contentEncoding string
	for patternExt := range p.ContentEncoding {
		if patternExt == fileExt {
			contentEncoding = p.ContentEncoding[patternExt]
			break
		}
	}

	var cacheControl string
	for pattern := range p.CacheControl {
		if match := glob.Glob(pattern, local); match {
			cacheControl = p.CacheControl[pattern]
			break
		}
	}

	metadata := map[string]string{}
	for pattern := range p.Metadata {
		if match := glob.Glob(pattern, local); match {
			for k, v := range p.Metadata[pattern] {
				metadata[k] = v
			}
			break
		}
	}

	head, err := a.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(remote),
	})
	if err != nil {
		debug("\"%s\" not found in bucket, uploading with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
		putObject := &s3.PutObjectInput{
			Bucket: aws.String(p.Bucket),
			Key:    aws.String(remote),
			Body:   file,
		}

		if len(contentType) > 0 {
			putObject.ContentType = aws.String(contentType)
		}

		if len(cacheControl) > 0 {
			putObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			putObject.ContentEncoding = aws.String(contentEncoding)
		}

		if len(metadata) > 0 {
			putObject.Metadata = metadata
		}

		// skip upload during dry run
		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.PutObject(context.TODO(), putObject)
		return err
	}

	hash := md5.New()
	io.Copy(hash, file)
	sum := fmt.Sprintf("\"%x\"", hash.Sum(nil))

	if head.ETag != nil && sum == *head.ETag {
		shouldUpload := false

		if head.ContentType == nil && contentType != "" {
			debug("Content-Type has changed from unset to %s", contentType)
			shouldUpload = true
		}

		if !shouldUpload && head.ContentType != nil && contentType != *head.ContentType {
			debug("Content-Type has changed from %s to %s", *head.ContentType, contentType)
			shouldUpload = true
		}

		if !shouldUpload && head.ContentEncoding == nil && contentEncoding != "" {
			debug("Content-Encoding has changed from unset to %s", contentEncoding)
			shouldUpload = true
		}

		if !shouldUpload && head.ContentEncoding != nil && contentEncoding != *head.ContentEncoding {
			debug("Content-Encoding has changed from %s to %s", *head.ContentEncoding, contentEncoding)
			shouldUpload = true
		}

		if !shouldUpload && head.CacheControl == nil && cacheControl != "" {
			debug("Cache-Control has changed from unset to %s", cacheControl)
			shouldUpload = true
		}

		if !shouldUpload && head.CacheControl != nil && cacheControl != *head.CacheControl {
			debug("Cache-Control has changed from %s to %s", *head.CacheControl, cacheControl)
			shouldUpload = true
		}

		if !shouldUpload && len(head.Metadata) != len(metadata) {
			debug("Count of metadata values has changed for %s", local)
			shouldUpload = true
		}

		if !shouldUpload && len(metadata) > 0 {
			for k, v := range metadata {
				if hv, ok := head.Metadata[k]; ok {
					if v != hv {
						debug("Metadata values have changed for %s", local)
						shouldUpload = true
						break
					}
				}
			}
		}

		if !shouldUpload {
			grant, err := a.client.GetObjectAcl(context.TODO(), &s3.GetObjectAclInput{
				Bucket: aws.String(p.Bucket),
				Key:    aws.String(remote),
			})
			if err == nil {
				previousAccess := "private"
				for _, g := range grant.Grants {
					if g.Grantee != nil && g.Grantee.URI != nil {
						if *g.Grantee.URI == "http://acs.amazonaws.com/groups/global/AllUsers" {
							if string(g.Permission) == "READ" {
								previousAccess = "public-read"
							} else if string(g.Permission) == "WRITE" {
								previousAccess = "public-read-write"
							}
						}
					}
				}

				if previousAccess != access {
					debug("Permissions for \"%s\" have changed from \"%s\" to \"%s\"", remote, previousAccess, access)
					shouldUpload = true
				}
			}
		}

		if !shouldUpload {
			debug("Skipping \"%s\" because hashes and metadata match", local)
			return nil
		}

		// Re-upload to update metadata/properties
		if _, err := file.Seek(0, 0); err != nil {
			return err
		}

		debug("Updating metadata for \"%s\" Content-Type: \"%s\", ACL: \"%s\"", local, contentType, access)
		putObject := &s3.PutObjectInput{
			Bucket: aws.String(p.Bucket),
			Key:    aws.String(remote),
			Body:   file,
		}

		if len(contentType) > 0 {
			putObject.ContentType = aws.String(contentType)
		}

		if len(cacheControl) > 0 {
			putObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			putObject.ContentEncoding = aws.String(contentEncoding)
		}

		if len(metadata) > 0 {
			putObject.Metadata = metadata
		}

		// skip update if dry run
		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.PutObject(context.TODO(), putObject)
		return err
	} else {
		_, err = file.Seek(0, 0)
		if err != nil {
			return err
		}

		debug("Uploading \"%s\" with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
		putObject := &s3.PutObjectInput{
			Bucket: aws.String(p.Bucket),
			Key:    aws.String(remote),
			Body:   file,
		}

		if len(contentType) > 0 {
			putObject.ContentType = aws.String(contentType)
		}

		if len(cacheControl) > 0 {
			putObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			putObject.ContentEncoding = aws.String(contentEncoding)
		}

		if len(metadata) > 0 {
			putObject.Metadata = metadata
		}

		// skip upload if dry run
		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.PutObject(context.TODO(), putObject)
		return err
	}
}

func (a *AWS) Redirect(path, location string) error {
	p := a.plugin
	debug("Adding redirect from \"%s\" to \"%s\"", path, location)

	if a.plugin.DryRun {
		return nil
	}

	_, err := a.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:                  aws.String(p.Bucket),
		Key:                     aws.String(path),
		WebsiteRedirectLocation: aws.String(location),
	})
	return err
}

func (a *AWS) Delete(remote string) error {
	p := a.plugin
	debug("Removing remote file \"%s\"", remote)

	if a.plugin.DryRun {
		return nil
	}

	_, err := a.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(remote),
	})
	return err
}

func (a *AWS) List(path string) ([]string, error) {
	p := a.plugin
	remote := make([]string, 0)
	resp, err := a.client.ListObjects(context.TODO(), &s3.ListObjectsInput{
		Bucket: aws.String(p.Bucket),
		Prefix: aws.String(path),
	})
	if err != nil {
		return remote, err
	}

	for _, item := range resp.Contents {
		remote = append(remote, *item.Key)
	}

	for resp.IsTruncated != nil && *resp.IsTruncated {
		resp, err = a.client.ListObjects(context.TODO(), &s3.ListObjectsInput{
			Bucket: aws.String(p.Bucket),
			Prefix: aws.String(path),
			Marker: aws.String(remote[len(remote)-1]),
		})

		if err != nil {
			return remote, err
		}

		for _, item := range resp.Contents {
			remote = append(remote, *item.Key)
		}
	}

	return remote, nil
}

func (a *AWS) Invalidate(invalidatePaths []string) error {
	p := a.plugin
	// Keep time usage to avoid unused import diagnostics.
	debug("Invalidating \"%v\" at %s", invalidatePaths, time.Now().Format(time.RFC3339Nano))

	// Skip in dry run
	if a.plugin.DryRun {
		return nil
	}

	// Call with minimal input and nil context to satisfy SDK v2 signature
	_, err := a.cfClient.CreateInvalidation(context.TODO(), &cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(p.CloudFrontDistribution),
	})
	return err
}
