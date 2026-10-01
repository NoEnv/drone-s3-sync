package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Plugin struct {
	Endpoint               string
	Key                    string
	Secret                 string
	Bucket                 string
	Region                 string
	Source                 string
	Target                 string
	Delete                 bool
	Access                 map[string]string
	CacheControl           map[string]string
	ContentType            map[string]string
	ContentEncoding        map[string]string
	Metadata               map[string]map[string]string
	Redirects              map[string]string
	CloudFrontDistribution string
	DryRun                 bool
	PathStyle              bool
	client                 AWS
	jobs                   []job
	MaxConcurrency         int
}

type job struct {
	local  string
	remote string
	paths  []string
	action string
}

type result struct {
	j   job
	err error
}

var MissingAwsValuesMessage = "Must set 'bucket'"

func (p *Plugin) Exec() error {
	err := p.sanitizeInputs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	p.jobs = make([]job, 0)
	p.client = NewAWS(p)

	p.createSyncJobs()
	p.createInvalidateJob()
	p.runJobs()
	return nil
}

func (p *Plugin) sanitizeInputs() error {
	if len(p.Bucket) == 0 {
		return errors.New(MissingAwsValuesMessage)
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	p.Source = filepath.Join(wd, p.Source)
	p.Target = strings.TrimPrefix(p.Target, string(os.PathSeparator))

	return nil
}

// listPrefix returns the S3 key prefix used to list the objects that belong to
// target. It always ends with a "/" (unless target is the bucket root), so
// syncing to "a/b" does not also match sibling prefixes such as "a/b-other/".
func listPrefix(target string) string {
	target = strings.Trim(target, "/")
	if target == "" {
		return ""
	}
	return target + "/"
}

// deleteCandidates returns the remote keys below target that have no
// counterpart in local (paths relative to target).
func deleteCandidates(remote []string, local []string, target string) []string {
	prefix := listPrefix(target)
	known := make(map[string]struct{}, len(local))
	for _, l := range local {
		known[l] = struct{}{}
	}

	candidates := make([]string, 0)
	for _, r := range remote {
		// never touch keys outside the target "directory"
		if !strings.HasPrefix(r, prefix) {
			continue
		}
		if _, found := known[strings.TrimPrefix(r, prefix)]; !found {
			candidates = append(candidates, r)
		}
	}
	return candidates
}

func (p *Plugin) createSyncJobs() {
	remote, err := p.client.List(listPrefix(p.Target))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	local := make([]string, 0)

	err = filepath.Walk(p.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		localPath := path
		if p.Source != "." {
			localPath = strings.TrimPrefix(path, p.Source)
			localPath = strings.TrimPrefix(localPath, string(os.PathSeparator))
		}
		local = append(local, localPath)
		p.jobs = append(p.jobs, job{
			local:  filepath.Join(p.Source, localPath),
			remote: filepath.Join(p.Target, localPath),
			action: "upload",
		})

		return nil
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for path, location := range p.Redirects {
		path = strings.TrimPrefix(path, string(os.PathSeparator))
		local = append(local, path)
		p.jobs = append(p.jobs, job{
			local:  path,
			remote: location,
			action: "redirect",
		})
	}
	if p.Delete {
		for _, r := range deleteCandidates(remote, local, p.Target) {
			p.jobs = append(p.jobs, job{
				local:  "",
				remote: r,
				action: "delete",
			})
		}
	}
}

func (p *Plugin) createInvalidateJob() {
	if len(p.CloudFrontDistribution) > 0 {
		p.jobs = append(p.jobs, job{
			paths:  []string{filepath.Join(string(os.PathSeparator), p.Target, "*")},
			action: "invalidateCloudFront",
		})
	}
}

func (p *Plugin) runJobs() {
	client := p.client
	jobChan := make(chan struct{}, p.MaxConcurrency)
	results := make(chan *result, len(p.jobs))
	var invalidateJob *job

	fmt.Printf("Synchronizing with bucket \"%s\"\n", p.Bucket)
	for _, j := range p.jobs {
		jobChan <- struct{}{}
		go func(j job) {
			var err error
			if j.action == "upload" {
				err = client.Upload(j.local, j.remote)
			} else if j.action == "redirect" {
				err = client.Redirect(j.local, j.remote)
			} else if j.action == "delete" {
				err = client.Delete(j.remote)
			} else if j.action == "invalidateCloudFront" {
				invalidateJob = &j
			} else {
				err = nil
			}
			results <- &result{j, err}
			<-jobChan
		}(j)
	}

	for range p.jobs {
		r := <-results
		if r.err != nil {
			fmt.Printf("ERROR: failed to %s %s to %s: %+v\n", r.j.action, r.j.local, r.j.remote, r.err)
			os.Exit(1)
		}
	}

	if invalidateJob != nil {
		fmt.Printf("Invalidating CloudFront distribution\n")
		err := client.Invalidate(invalidateJob.paths)
		if err != nil {
			fmt.Printf("ERROR: failed to %s %s: %+v\n", invalidateJob.action, invalidateJob.paths, err)
			os.Exit(1)
		}
	}
}

func debug(format string, args ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		fmt.Printf(format+"\n", args...)
	}
}
