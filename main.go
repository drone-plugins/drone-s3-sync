package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/drone/drone-go/plugin"
)

func main() {
	vargs := PluginArgs{}
	workspace := drone.Workspace{}

	plugin.Param("vargs", &vargs)
	plugin.Param("workspace", &workspace)
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
	vargs.Source = filepath.Join(workspace.Path, vargs.Source)

	if strings.HasPrefix(vargs.Target, "/") {
		vargs.Target = vargs.Target[1:]
	}

	client := NewAWS(vargs)
	err := client.List(vargs.Target)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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

func debug(format string, args ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		fmt.Printf(format, args...)
	}
}
