package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ryanuber/go-glob"
)

type AWS struct {
	client   *s3.S3
	cfClient *cloudfront.CloudFront
	remote   []string
	local    []string
	plugin   *Plugin
}

func NewAWS(p *Plugin) AWS {

	sessCfg := &aws.Config{
		S3ForcePathStyle: aws.Bool(p.PathStyle),
		Region:           aws.String(p.Region),
	}

	if p.Endpoint != "" {
		sessCfg.Endpoint = &p.Endpoint
		sessCfg.DisableSSL = aws.Bool(strings.HasPrefix(p.Endpoint, "http://"))
	}

	// allowing to use the instance role or provide a key and secret
	if p.Key != "" && p.Secret != "" {
		sessCfg.Credentials = credentials.NewStaticCredentials(p.Key, p.Secret, "")
	}

	sess := session.New(sessCfg)

	c := s3.New(sess)
	cf := cloudfront.New(sess)
	r := make([]string, 1, 1)
	l := make([]string, 1, 1)

	return AWS{c, cf, r, l, p}
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
		if match := glob.Glob(pattern, local); match == true {
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
		if match := glob.Glob(pattern, local); match == true {
			cacheControl = p.CacheControl[pattern]
			break
		}
	}

	metadata := map[string]*string{}
	for pattern := range p.Metadata {
		if match := glob.Glob(pattern, local); match == true {
			for k, v := range p.Metadata[pattern] {
				metadata[k] = aws.String(v)
			}
			break
		}
	}

	head, err := a.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(remote),
	})
	if err != nil && err.(awserr.Error).Code() != "404" {
		if err.(awserr.Error).Code() == "404" {
			return err
		}

		debug("\"%s\" not found in bucket, uploading with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
		var putObject = &s3.PutObjectInput{
			Bucket:      aws.String(p.Bucket),
			Key:         aws.String(remote),
			Body:        file,
			ContentType: aws.String(contentType),
			ACL:         aws.String(access),
			Metadata:    metadata,
		}

		if len(cacheControl) > 0 {
			putObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			putObject.ContentEncoding = aws.String(contentEncoding)
		}

		// skip upload during dry run
		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.PutObject(putObject)
		return err
	}

	hash := md5.New()
	io.Copy(hash, file)
	sum := fmt.Sprintf("\"%x\"", hash.Sum(nil))

	if sum == *head.ETag {
		shouldCopy := false

		if head.ContentType == nil && contentType != "" {
			debug("Content-Type has changed from unset to %s", contentType)
			shouldCopy = true
		}

		if !shouldCopy && head.ContentType != nil && contentType != *head.ContentType {
			debug("Content-Type has changed from %s to %s", *head.ContentType, contentType)
			shouldCopy = true
		}

		if !shouldCopy && head.ContentEncoding == nil && contentEncoding != "" {
			debug("Content-Encoding has changed from unset to %s", contentEncoding)
			shouldCopy = true
		}

		if !shouldCopy && head.ContentEncoding != nil && contentEncoding != *head.ContentEncoding {
			debug("Content-Encoding has changed from %s to %s", *head.ContentEncoding, contentEncoding)
			shouldCopy = true
		}

		if !shouldCopy && head.CacheControl == nil && cacheControl != "" {
			debug("Cache-Control has changed from unset to %s", cacheControl)
			shouldCopy = true
		}

		if !shouldCopy && head.CacheControl != nil && cacheControl != *head.CacheControl {
			debug("Cache-Control has changed from %s to %s", *head.CacheControl, cacheControl)
			shouldCopy = true
		}

		if !shouldCopy && len(head.Metadata) != len(metadata) {
			debug("Count of metadata values has changed for %s", local)
			shouldCopy = true
		}

		if !shouldCopy && len(metadata) > 0 {
			for k, v := range metadata {
				if hv, ok := head.Metadata[k]; ok {
					if *v != *hv {
						debug("Metadata values have changed for %s", local)
						shouldCopy = true
						break
					}
				}
			}
		}

		if !shouldCopy {
			grant, err := a.client.GetObjectAcl(&s3.GetObjectAclInput{
				Bucket: aws.String(p.Bucket),
				Key:    aws.String(remote),
			})
			if err != nil {
				return err
			}

			previousAccess := "private"
			for _, g := range grant.Grants {
				gt := *g.Grantee
				if gt.URI != nil {
					if *gt.URI == "http://acs.amazonaws.com/groups/global/AllUsers" {
						if *g.Permission == "READ" {
							previousAccess = "public-read"
						} else if *g.Permission == "WRITE" {
							previousAccess = "public-read-write"
						}
					} else if *gt.URI == "http://acs.amazonaws.com/groups/global/AllUsers" {
						if *g.Permission == "READ" {
							previousAccess = "authenticated-read"
						}
					}
				}
			}

			if previousAccess != access {
				debug("Permissions for \"%s\" have changed from \"%s\" to \"%s\"", remote, previousAccess, access)
				shouldCopy = true
			}
		}

		if !shouldCopy {
			debug("Skipping \"%s\" because hashes and metadata match", local)
			return nil
		}

		debug("Updating metadata for \"%s\" Content-Type: \"%s\", ACL: \"%s\"", local, contentType, access)
		var copyObject = &s3.CopyObjectInput{
			Bucket:            aws.String(p.Bucket),
			Key:               aws.String(remote),
			CopySource:        aws.String(fmt.Sprintf("%s/%s", p.Bucket, remote)),
			ACL:               aws.String(access),
			ContentType:       aws.String(contentType),
			Metadata:          metadata,
			MetadataDirective: aws.String("REPLACE"),
		}

		if len(cacheControl) > 0 {
			copyObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			copyObject.ContentEncoding = aws.String(contentEncoding)
		}

		// skip update if dry run
		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.CopyObject(copyObject)
		return err
	} else {
		_, err = file.Seek(0, 0)
		if err != nil {
			return err
		}

		debug("Uploading \"%s\" with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
		var putObject = &s3.PutObjectInput{
			Bucket:      aws.String(p.Bucket),
			Key:         aws.String(remote),
			Body:        file,
			ContentType: aws.String(contentType),
			ACL:         aws.String(access),
			Metadata:    metadata,
		}

		if len(cacheControl) > 0 {
			putObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			putObject.ContentEncoding = aws.String(contentEncoding)
		}

		// skip upload if dry run
		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.PutObject(putObject)
		return err
	}
}

func (a *AWS) Redirect(path, location string) error {
	p := a.plugin
	debug("Adding redirect from \"%s\" to \"%s\"", path, location)

	if a.plugin.DryRun {
		return nil
	}

	_, err := a.client.PutObject(&s3.PutObjectInput{
		Bucket:                  aws.String(p.Bucket),
		Key:                     aws.String(path),
		ACL:                     aws.String("public-read"),
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

	_, err := a.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(remote),
	})
	return err
}

func (a *AWS) List(path string) ([]string, error) {
	p := a.plugin
	remote := make([]string, 1, 1)
	resp, err := a.client.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(p.Bucket),
		Prefix: aws.String(path),
	})
	if err != nil {
		return remote, err
	}

	for _, item := range resp.Contents {
		remote = append(remote, *item.Key)
	}

	for *resp.IsTruncated {
		resp, err = a.client.ListObjects(&s3.ListObjectsInput{
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

func (a *AWS) Invalidate(invalidatePath string) error {
	p := a.plugin
	debug("Invalidating \"%s\"", invalidatePath)
	_, err := a.cfClient.CreateInvalidation(&cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(p.CloudFrontDistribution),
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: aws.String(time.Now().Format(time.RFC3339Nano)),
			Paths: &cloudfront.Paths{
				Quantity: aws.Int64(1),
				Items: []*string{
					aws.String(invalidatePath),
				},
			},
		},
	})
	return err
}
