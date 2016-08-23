package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/joho/godotenv"
	"github.com/urfave/cli"
)

var version string // build number set at compile-time

func main() {
	app := cli.NewApp()
	app.Name = "s3 sync plugin"
	app.Usage = "s3 sync plugin"
	app.Action = run
	app.Version = version
	app.Flags = []cli.Flag{
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
			EnvVar: "PLUGIN_ACCESS",
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
		cli.StringFlag{
			Name:  "env-file",
			Usage: "source env file",
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) error {
	if c.String("env-file") != "" {
		_ = godotenv.Load(c.String("env-file"))
	}

	plugin := Plugin{
		Key:                    c.String("access-key"),
		Secret:                 c.String("secret-key"),
		Bucket:                 c.String("bucket"),
		Region:                 c.String("region"),
		Source:                 c.String("source"),
		Target:                 c.String("target"),
		Delete:                 c.Bool("delete"),
		Access:                 c.Generic("access").(*StringMapFlag).Get(),
		ContentType:            c.Generic("content-type").(*StringMapFlag).Get(),
		ContentEncoding:        c.Generic("content-encoding").(*StringMapFlag).Get(),
		Metadata:               c.Generic("metadata").(*DeepStringMapFlag).Get(),
		Redirects:              c.Generic("redirects").(*MapFlag).Get(),
		CloudFrontDistribution: c.String("cloudfront-distribution"),
	}

	return plugin.Exec()
}
