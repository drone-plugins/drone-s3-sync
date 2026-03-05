package main

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/ryanuber/go-glob"
)

type AWS struct {
	client   *s3.Client
	cfClient *cloudfront.Client
	remote   []string
	local    []string
	plugin   *Plugin
}

func NewAWS(p *Plugin) AWS {
	ctx := context.Background()

	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(p.Region),
	}

	if p.Key != "" && p.Secret != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(p.Key, p.Secret, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		panic(fmt.Sprintf("unable to load AWS config: %v", err))
	}

	s3Opts := []func(*s3.Options){}
	if p.Endpoint != "" {
		endpoint := normalizeEndpoint(p.Endpoint)
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = p.PathStyle
			// S3-compatible services (MinIO, Spaces, B2, etc.) may not support the
			// CRC32 checksums that SDK v2 sends by default with PutObject.
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		})
	} else if p.PathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	c := s3.NewFromConfig(cfg, s3Opts...)
	cf := cloudfront.NewFromConfig(cfg)
	r := make([]string, 1)
	l := make([]string, 1)

	return AWS{c, cf, r, l, p}
}

func normalizeEndpoint(endpoint string) string {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return "https://" + endpoint
	}
	return endpoint
}

func (a *AWS) Upload(local, remote string) error {
	ctx := context.Background()
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

	head, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(remote),
	})
	if err != nil {
		var apiErr smithy.APIError
		isNotFound := false
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "404" || apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey" {
				isNotFound = true
			}
		}
		var nsb *s3types.NoSuchKey
		if errors.As(err, &nsb) {
			isNotFound = true
		}

		if !isNotFound {
			debug("\"%s\" not found in bucket, uploading with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
			var putObject = &s3.PutObjectInput{
				Bucket:      aws.String(p.Bucket),
				Key:         aws.String(remote),
				Body:        file,
				ContentType: aws.String(contentType),
				ACL:         s3types.ObjectCannedACL(access),
				Metadata:    metadata,
			}

			if len(cacheControl) > 0 {
				putObject.CacheControl = aws.String(cacheControl)
			}

			if len(contentEncoding) > 0 {
				putObject.ContentEncoding = aws.String(contentEncoding)
			}

			if a.plugin.DryRun {
				return nil
			}

			_, err = a.client.PutObject(ctx, putObject)
			return err
		}

		debug("\"%s\" not found in bucket, uploading with Content-Type \"%s\" and permissions \"%s\"", local, contentType, access)
		var putObject = &s3.PutObjectInput{
			Bucket:      aws.String(p.Bucket),
			Key:         aws.String(remote),
			Body:        file,
			ContentType: aws.String(contentType),
			ACL:         s3types.ObjectCannedACL(access),
			Metadata:    metadata,
		}

		if len(cacheControl) > 0 {
			putObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			putObject.ContentEncoding = aws.String(contentEncoding)
		}

		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.PutObject(ctx, putObject)
		return err
	}

	hash := md5.New()
	_, _ = io.Copy(hash, file)
	sum := fmt.Sprintf("\"%x\"", hash.Sum(nil))

	if head.ETag != nil && sum == *head.ETag {
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
					if v != hv {
						debug("Metadata values have changed for %s", local)
						shouldCopy = true
						break
					}
				}
			}
		}

		if !shouldCopy {
			debug("Retrieving ACL for \"%s\"", local)
			grant, err := a.client.GetObjectAcl(ctx, &s3.GetObjectAclInput{
				Bucket: aws.String(p.Bucket),
				Key:    aws.String(remote),
			})
			if err != nil {
				return err
			}

			previousAccess := "private"
			for _, g := range grant.Grants {
				gt := g.Grantee
				if gt.URI != nil {
					if *gt.URI == "http://acs.amazonaws.com/groups/global/AllUsers" {
						if g.Permission == s3types.PermissionRead {
							previousAccess = "public-read"
						} else if g.Permission == s3types.PermissionWrite {
							previousAccess = "public-read-write"
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
			ACL:               s3types.ObjectCannedACL(access),
			ContentType:       aws.String(contentType),
			Metadata:          metadata,
			MetadataDirective: s3types.MetadataDirectiveReplace,
		}

		if len(cacheControl) > 0 {
			copyObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			copyObject.ContentEncoding = aws.String(contentEncoding)
		}

		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.CopyObject(ctx, copyObject)
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
			ACL:         s3types.ObjectCannedACL(access),
			Metadata:    metadata,
		}

		if len(cacheControl) > 0 {
			putObject.CacheControl = aws.String(cacheControl)
		}

		if len(contentEncoding) > 0 {
			putObject.ContentEncoding = aws.String(contentEncoding)
		}

		if a.plugin.DryRun {
			return nil
		}

		_, err = a.client.PutObject(ctx, putObject)
		return err
	}
}

func (a *AWS) Redirect(path, location string) error {
	ctx := context.Background()
	p := a.plugin
	debug("Adding redirect from \"%s\" to \"%s\"", path, location)

	if a.plugin.DryRun {
		return nil
	}

	_, err := a.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:                  aws.String(p.Bucket),
		Key:                     aws.String(path),
		ACL:                     s3types.ObjectCannedACLPublicRead,
		WebsiteRedirectLocation: aws.String(location),
	})
	return err
}

func (a *AWS) Delete(remote string) error {
	ctx := context.Background()
	p := a.plugin
	debug("Removing remote file \"%s\"", remote)

	if a.plugin.DryRun {
		return nil
	}

	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(remote),
	})
	return err
}

func (a *AWS) List(path string) ([]string, error) {
	ctx := context.Background()
	p := a.plugin
	remote := make([]string, 1)
	resp, err := a.client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: aws.String(p.Bucket),
		Prefix: aws.String(path),
	})
	if err != nil {
		return remote, err
	}

	for _, item := range resp.Contents {
		remote = append(remote, *item.Key)
	}

	for aws.ToBool(resp.IsTruncated) {
		resp, err = a.client.ListObjects(ctx, &s3.ListObjectsInput{
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
	ctx := context.Background()
	p := a.plugin
	debug("Invalidating \"%s\"", invalidatePath)
	_, err := a.cfClient.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(p.CloudFrontDistribution),
		InvalidationBatch: &cftypes.InvalidationBatch{
			CallerReference: aws.String(time.Now().Format(time.RFC3339Nano)),
			Paths: &cftypes.Paths{
				Quantity: aws.Int32(1),
				Items: []string{
					invalidatePath,
				},
			},
		},
	})
	return err
}
