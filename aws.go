package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ryanuber/go-glob"
)

type AWS struct {
	client *s3.S3
	remote []string
	local  []string
	vargs  PluginArgs
}

func NewAWS(vargs PluginArgs) AWS {
	sess := session.New(&aws.Config{
		Credentials: credentials.NewStaticCredentials(vargs.Key, vargs.Secret, ""),
		Region:      aws.String(vargs.Region),
	})
	c := s3.New(sess)
	r := make([]string, 1, 1)
	l := make([]string, 1, 1)

	return AWS{c, r, l, vargs}
}

func (a *AWS) Upload(local, remote string) error {
	if local == "" {
		return nil
	}

	file, err := os.Open(local)
	if err != nil {
		return err
	}

	defer file.Close()

	access := ""
	if a.vargs.Access.IsString() {
		access = a.vargs.Access.String()
	} else if !a.vargs.Access.IsEmpty() {
		accessMap := a.vargs.Access.Map()
		for pattern := range accessMap {
			if match := glob.Glob(pattern, local); match == true {
				access = accessMap[pattern]
				break
			}
		}
	}

	if access == "" {
		access = "private"
	}

	fileExt := filepath.Ext(local)
	var contentType string
	if a.vargs.ContentType.IsString() {
		contentType = a.vargs.ContentType.String()
	} else if !a.vargs.ContentType.IsEmpty() {
		contentMap := a.vargs.ContentType.Map()
		for patternExt := range contentMap {
			if patternExt == fileExt {
				contentType = contentMap[patternExt]
				break
			}
		}
	}

	metadata := map[string]*string{}
	vmap := a.vargs.Metadata.Map()
	if len(vmap) > 0 {
		for pattern := range vmap {
			if match := glob.Glob(pattern, local); match == true {
				for k, v := range vmap[pattern] {
					metadata[k] = aws.String(v)
				}
				break
			}
		}
	}

	if contentType == "" {
		contentType = mime.TypeByExtension(fileExt)
	}

	head, err := a.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(a.vargs.Bucket),
		Key:    aws.String(remote),
	})
	if err != nil && err.(awserr.Error).Code() != "404" {
		if err.(awserr.Error).Code() == "404" {
			return err
		}

		debug("Uploading \"%s\" with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
		_, err = a.client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(a.vargs.Bucket),
			Key:         aws.String(remote),
			Body:        file,
			ContentType: aws.String(contentType),
			ACL:         aws.String(access),
			Metadata:    metadata,
		})
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
				Bucket: aws.String(a.vargs.Bucket),
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
		_, err = a.client.CopyObject(&s3.CopyObjectInput{
			Bucket:            aws.String(a.vargs.Bucket),
			Key:               aws.String(remote),
			CopySource:        aws.String(fmt.Sprintf("%s/%s", a.vargs.Bucket, remote)),
			ACL:               aws.String(access),
			ContentType:       aws.String(contentType),
			Metadata:          metadata,
			MetadataDirective: aws.String("REPLACE"),
		})
		return err
	} else {
		debug("Uploading \"%s\" with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
		_, err = a.client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(a.vargs.Bucket),
			Key:         aws.String(remote),
			Body:        file,
			ContentType: aws.String(contentType),
			ACL:         aws.String(access),
			Metadata:    metadata,
		})
		return err
	}
}

func (a *AWS) Redirect(path, location string) error {
	debug("Adding redirect from \"%s\" to \"%s\"", path, location)
	_, err := a.client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(a.vargs.Bucket),
		Key:    aws.String(path),
		ACL:    aws.String("public-read"),
		WebsiteRedirectLocation: aws.String(location),
	})
	return err
}

func (a *AWS) Delete(remote string) error {
	debug("Removing remote file \"%s\"", remote)
	_, err := a.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(a.vargs.Bucket),
		Key:    aws.String(remote),
	})
	return err
}

func (a *AWS) List(path string) ([]string, error) {
	remote := make([]string, 1, 1)
	resp, err := a.client.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(a.vargs.Bucket),
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
			Bucket: aws.String(a.vargs.Bucket),
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
