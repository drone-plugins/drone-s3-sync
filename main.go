package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/drone/drone-plugin-go/plugin"
)

type S3 struct {
	Key    string `json:"access_key"`
	Secret string `json:"secret_key"`
	Bucket string `json:"bucket"`

	// us-east-1
	// us-west-1
	// us-west-2
	// eu-west-1
	// ap-southeast-1
	// ap-southeast-2
	// ap-northeast-1
	// sa-east-1
	Region string `json:"region"`

	// Indicates the files ACL, which should be one
	// of the following:
	//     private
	//     public-read
	//     public-read-write
	//     authenticated-read
	//     bucket-owner-read
	//     bucket-owner-full-control
	Access string `json:"acl"`

	// Copies the files from the specified directory.
	// Regexp matching will apply to match multiple
	// files
	//
	// Examples:
	//    /path/to/file
	//    /path/to/*.txt
	//    /path/to/*/*.txt
	//    /path/to/**
	Source string `json:"source"`
	Target string `json:"target"`

	// Include or exclude all files or objects from the command
	// that matches the specified pattern.
	Include string `json:"include"`
	Exclude string `json:"exclude"`

	// Files that exist in the destination but not in the source
	// are deleted during sync.
	Delete bool `json:"delete"`

	// Specify an explicit content type for this operation. This
	// value overrides any guessed mime types.
	ContentType string `json:"content_type"`
}

func main() {
	workspace := plugin.Workspace{}
	vargs := S3{}

	plugin.Param("workspace", &workspace)
	plugin.Param("vargs", &vargs)
	plugin.MustParse()

	// skip if AWS key or SECRET are empty. A good example for this would
	// be forks building a project. S3 might be configured in the source
	// repo, but not in the fork
	if len(vargs.Key) == 0 || len(vargs.Secret) == 0 {
		return
	}

	// make sure a default region is set
	if len(vargs.Region) == 0 {
		vargs.Region = "us-east-1"
	}

	// make sure a default access is set
	// let's be conservative and assume private
	if len(vargs.Access) == 0 {
		vargs.Access = "private"
	}

	// make sure a default source is set
	if len(vargs.Source) == 0 {
		vargs.Source = "."
	}

	// if the target starts with a "/" we need
	// to remove it, otherwise we might adding
	// a 3rd slash to s3://
	if strings.HasPrefix(vargs.Target, "/") {
		vargs.Target = vargs.Target[1:]
	}
	vargs.Target = fmt.Sprintf("s3://%s/%s", vargs.Bucket, vargs.Target)

	cmd := command(vargs)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "AWS_ACCESS_KEY_ID="+vargs.Key)
	cmd.Env = append(cmd.Env, "AWS_SECRET_ACCESS_KEY="+vargs.Secret)
	cmd.Dir = workspace.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	trace(cmd)

	// run the command and exit if failed.
	err := cmd.Run()
	if err != nil {
		os.Exit(1)
	}
}

// command is a helper function that returns the command
// and arguments to upload to aws from the command line.
func command(s S3) *exec.Cmd {

	// command line args
	args := []string{
		"s3",
		"sync",
		s.Source,
		s.Target,
		"--acl",
		s.Access,
		"--region",
		s.Region,
	}

	// append delete flag if specified
	if s.Delete {
		args = append(args, "--delete")
	}
	// appends exclude flag if specified
	if len(s.Exclude) != 0 {
		args = append(args, "--exclude")
		args = append(args, s.Exclude)
	}
	// append include flag if specified
	if len(s.Include) != 0 {
		args = append(args, "--include")
		args = append(args, s.Include)
	}
	// appends content-type if specified
	if len(s.ContentType) != 0 {
		args = append(args, "--content-type")
		args = append(args, s.ContentType)
	}

	return exec.Command("aws", args...)
}

// trace writes each command to standard error (preceded by a ‘$ ’) before it
// is executed. Used for debugging your build.
func trace(cmd *exec.Cmd) {
	fmt.Println("$", strings.Join(cmd.Args, " "))
}
