package main

import (
	"fmt"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/joho/godotenv"
	"github.com/urfave/cli"

	"github.com/pquerna/otp/totp"
)

var build = "0" // build number set at compile-time

func main() {
	app := cli.NewApp()
	app.Name = "s3 sync plugin"
	app.Usage = "s3 sync plugin"
	app.Action = run
	app.Version = fmt.Sprintf("1.1.0+%s", build)
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "endpoint",
			Usage:  "endpoint for the s3 connection",
			EnvVar: "PLUGIN_ENDPOINT,S3_SYNC_ENDPOINT,S3_ENDPOINT",
		},
		cli.StringFlag{
			Name:   "access-key",
			Usage:  "aws access key",
			EnvVar: "PLUGIN_ACCESS_KEY,AWS_ACCESS_KEY_ID",
		},
		cli.StringFlag{
			Name:   "secret-key",
			Usage:  "aws secret key",
			EnvVar: "PLUGIN_SECRET_KEY,AWS_SECRET_ACCESS_KEY",
		},
		cli.StringFlag{
			Name:   "secret-mfa-key",
			Usage:  "aws mfa key",
			EnvVar: "PLUGIN_MFA_KEY,AWS_MFA_KEY",
		},
		cli.StringFlag{
			Name:   "secret-mfa-serial",
			Usage:  "aws mfa serial",
			EnvVar: "PLUGIN_MFA_SERIAL,AWS_MFA_SERIAL",
		},
		cli.BoolFlag{
			Name:   "path-style",
			Usage:  "use path style for bucket paths",
			EnvVar: "PLUGIN_PATH_STYLE",
		},
		cli.StringFlag{
			Name:   "bucket",
			Usage:  "name of bucket",
			EnvVar: "PLUGIN_BUCKET",
		},
		cli.StringFlag{
			Name:   "region",
			Usage:  "aws region",
			Value:  "us-east-1",
			EnvVar: "PLUGIN_REGION",
		},
		cli.StringFlag{
			Name:   "source",
			Usage:  "upload source path",
			Value:  ".",
			EnvVar: "PLUGIN_SOURCE",
		},
		cli.StringFlag{
			Name:   "target",
			Usage:  "target path",
			Value:  "/",
			EnvVar: "PLUGIN_TARGET",
		},
		cli.BoolFlag{
			Name:   "delete",
			Usage:  "delete locally removed files from the target",
			EnvVar: "PLUGIN_DELETE",
		},
		cli.GenericFlag{
			Name:   "access",
			Usage:  "access control settings",
			EnvVar: "PLUGIN_ACCESS,PLUGIN_ACL",
			Value:  &StringMapFlag{},
		},
		cli.GenericFlag{
			Name:   "content-type",
			Usage:  "content-type settings for uploads",
			EnvVar: "PLUGIN_CONTENT_TYPE",
			Value:  &StringMapFlag{},
		},
		cli.GenericFlag{
			Name:   "content-encoding",
			Usage:  "content-encoding settings for uploads",
			EnvVar: "PLUGIN_CONTENT_ENCODING",
			Value:  &StringMapFlag{},
		},
		cli.GenericFlag{
			Name:   "cache-control",
			Usage:  "cache-control settings for uploads",
			EnvVar: "PLUGIN_CACHE_CONTROL",
			Value:  &StringMapFlag{},
		},
		cli.GenericFlag{
			Name:   "metadata",
			Usage:  "additional metadata for uploads",
			EnvVar: "PLUGIN_METADATA",
			Value:  &DeepStringMapFlag{},
		},
		cli.GenericFlag{
			Name:   "redirects",
			Usage:  "redirects to create",
			EnvVar: "PLUGIN_REDIRECTS",
			Value:  &MapFlag{},
		},
		cli.StringFlag{
			Name:   "cloudfront-distribution",
			Usage:  "id of cloudfront distribution to invalidate",
			EnvVar: "PLUGIN_CLOUDFRONT_DISTRIBUTION",
		},
		cli.BoolFlag{
			Name:   "dry-run",
			Usage:  "dry run disables api calls",
			EnvVar: "DRY_RUN,PLUGIN_DRY_RUN",
		},
		cli.StringFlag{
			Name:  "env-file",
			Usage: "source env file",
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	if c.String("env-file") != "" {
		_ = godotenv.Load(c.String("env-file"))
	}

	plugin := Plugin{
		Endpoint:               c.String("endpoint"),
		PathStyle:              c.Bool("path-style"),
		Key:                    c.String("access-key"),
		Secret:                 c.String("secret-key"),
		Token:                  "",
		Bucket:                 c.String("bucket"),
		Region:                 c.String("region"),
		Source:                 c.String("source"),
		Target:                 c.String("target"),
		Delete:                 c.Bool("delete"),
		Access:                 c.Generic("access").(*StringMapFlag).Get(),
		CacheControl:           c.Generic("cache-control").(*StringMapFlag).Get(),
		ContentType:            c.Generic("content-type").(*StringMapFlag).Get(),
		ContentEncoding:        c.Generic("content-encoding").(*StringMapFlag).Get(),
		Metadata:               c.Generic("metadata").(*DeepStringMapFlag).Get(),
		Redirects:              c.Generic("redirects").(*MapFlag).Get(),
		CloudFrontDistribution: c.String("cloudfront-distribution"),
		DryRun:                 c.Bool("dry-run"),
	}

	mfaKey := c.String("secret-mfa-key")
	mfaSerial := c.String("secret-mfa-serial")
	if len(mfaKey) != 0 && len(mfaSerial) != 0 {
		log.Printf("Authentication by MFA")
		setSessionToken(&plugin, mfaKey, mfaSerial)
	}

	return plugin.Exec()
}

func setSessionToken(plugin *Plugin, mfaKey string, mfaSerial string) {
	key, err := totp.GenerateCode(mfaKey, time.Now())
	if err != nil {
		log.Fatalf("error in generating one time password: %v", err)
	}

	stsService := sts.New(session.New(&aws.Config{
		Region:      &plugin.Region,
		Credentials: credentials.NewStaticCredentials(plugin.Key, plugin.Secret, ""),
	}))
	input := &sts.GetSessionTokenInput{
		DurationSeconds: aws.Int64(3600),
		SerialNumber:    aws.String(mfaSerial),
		TokenCode:       aws.String(key),
	}

	result, err := stsService.GetSessionToken(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case sts.ErrCodeRegionDisabledException:
				log.Fatal(sts.ErrCodeRegionDisabledException, aerr.Error())
			default:
				log.Fatalf("error during getting session token (aws error): %v", aerr)
			}
		} else {
			log.Fatalf("error during getting session token: %v", err)
		}
		return
	}

	plugin.Key = *result.Credentials.AccessKeyId
	plugin.Secret = *result.Credentials.SecretAccessKey
	plugin.Token = *result.Credentials.SessionToken
}
