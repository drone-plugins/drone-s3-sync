package main

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func newAppWithAwsVars() *app {
    a := newApp()
    a.vargs.Key = "AAAA"
    a.vargs.Secret = "AAAA"
    a.vargs.Bucket = "AAAA"
    
    return a
}

func TestSanitizeInputs(t *testing.T) {
    a := newApp()
    err := a.sanitizeInputs()
    assert.EqualError(t, err, MissingAwsValuesMessage)
    
    a = newAppWithAwsVars()
    a.workspace.Path = "foo/bar"
    a.vargs.Target = "/remote/foo"
    a.sanitizeInputs()
    
    assert.EqualValues(t, "foo/bar", a.vargs.Source, "Source should default to workspace.Path")
    assert.EqualValues(t, "us-east-1", a.vargs.Region, "Region should default to 'us-east'")
    assert.EqualValues(t, "remote/foo", a.vargs.Target, "Target should have first slash stripped")
    
    a = newAppWithAwsVars()
    a.workspace.Path = "foo/bar"
    a.vargs.Source = "some/folder"
    a.vargs.Target = "remote/foo"
    
    a.sanitizeInputs()
    
    assert.EqualValues(t, "foo/bar/some/folder", a.vargs.Source, "Source should combine workspace.Path and specified Source")
    assert.EqualValues(t, "remote/foo", a.vargs.Target, "Target should have first slash stripped")
    
}