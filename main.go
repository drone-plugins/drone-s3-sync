package main

import (
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/drone-go/plugin"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
)

type AWS struct {
	client *s3.S3
	bucket *s3.Bucket
	remote []string
	local  []string
	vargs  PluginArgs
}

type StringMap struct {
	parts map[string]string
}

func (e *StringMap) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	p := map[string]string{}
	if err := json.Unmarshal(b, &p); err != nil {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p["_string_"] = s
	}

	e.parts = p
	return nil
}

func (e *StringMap) IsEmpty() bool {
	if e == nil || len(e.parts) == 0 {
		return true
	}

	return false
}

func (e *StringMap) IsString() bool {
	if e.IsEmpty() || len(e.parts) != 1 {
		return false
	}

	_, ok := e.parts["_string_"]
	return ok
}

func (e *StringMap) String() string {
	if e.IsEmpty() || !e.IsString() {
		return ""
	}

	return e.parts["_string_"]
}

func (e *StringMap) Map() map[string]string {
	if e.IsEmpty() || e.IsString() {
		return map[string]string{}
	}

	return e.parts
}

type PluginArgs struct {
	Key         string    `json:"access_key"`
	Secret      string    `json:"secret_key"`
	Bucket      string    `json:"bucket"`
	Region      string    `json:"region"`
	Source      string    `json:"source"`
	Target      string    `json:"target"`
	Delete      bool      `json:"delete"`
	Access      StringMap `json:"acl"`
	ContentType StringMap `json:"content_type"`
}

func NewClient(vargs PluginArgs) AWS {
	auth := aws.Auth{AccessKey: vargs.Key, SecretKey: vargs.Secret}
	region := aws.Regions[vargs.Region]
	client := s3.New(auth, region)
	bucket := client.Bucket(vargs.Bucket)
	remote := make([]string, 1, 1)
	local := make([]string, 1, 1)

	aws := AWS{client, bucket, remote, local, vargs}
	return aws
}

func (aws *AWS) visit(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if path == "." {
		return nil
	}

	if info.IsDir() {
		return nil
	}

	aws.local = append(aws.local, path)
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer file.Close()

	var access s3.ACL
	if aws.vargs.Access.IsString() {
		access = s3.ACL(aws.vargs.Access.String())
	} else if !aws.vargs.Access.IsEmpty() {
		accessMap := aws.vargs.Access.Map()
		for pattern := range accessMap {
			if match, _ := filepath.Match(pattern, path); match == true {
				access = s3.ACL(accessMap[pattern])
				break
			}
		}
	}

	if access == "" {
		access = s3.ACL("private")
	}

	fileExt := filepath.Ext(path)
	var contentType string
	if aws.vargs.ContentType.IsString() {
		contentType = aws.vargs.ContentType.String()
	} else if !aws.vargs.ContentType.IsEmpty() {
		contentMap := aws.vargs.ContentType.Map()
		for patternExt := range contentMap {
			if patternExt == fileExt {
				contentType = contentMap[patternExt]
				break
			}
		}
	}

	if contentType == "" {
		contentType = mime.TypeByExtension(fileExt)
	}

	fmt.Printf("Uploading %s with Content-Type %s and permissions %s\n", path, contentType, access)
	err = aws.bucket.PutReader(path, file, info.Size(), contentType, access)
	if err != nil {
		return err
	}

	return nil
}

func (aws *AWS) List(path string) (*s3.ListResp, error) {
	return aws.bucket.List(path, "", "", 10000)
}

func (aws *AWS) Cleanup() error {
	for _, remote := range aws.remote {
		found := false
		for _, local := range aws.local {
			if local == remote {
				found = true
				break
			}
		}

		if !found {
			fmt.Println("Removing remote file ", remote)
			err := aws.bucket.Del(remote)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	vargs := PluginArgs{}

	plugin.Param("vargs", &vargs)
	if err := plugin.Parse(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(vargs.Key) == 0 || len(vargs.Secret) == 0 || len(vargs.Bucket) == 0 {
		return
	}

	if len(vargs.Region) == 0 {
		vargs.Region = "us-east-1"
	}

	if len(vargs.Source) == 0 {
		vargs.Source = "."
	}

	if strings.HasPrefix(vargs.Target, "/") {
		vargs.Target = vargs.Target[1:]
	}

	if vargs.Target != "" && !strings.HasSuffix(vargs.Target, "/") {
		vargs.Target = fmt.Sprintf("%s/", vargs.Target)
	}

	client := NewClient(vargs)

	resp, err := client.List(vargs.Target)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, item := range resp.Contents {
		client.remote = append(client.remote, item.Key)
	}

	err = filepath.Walk(vargs.Source, client.visit)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if vargs.Delete {
		err = client.Cleanup()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}
