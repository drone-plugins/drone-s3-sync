Use the S3 sync plugin to synchronize files and folders with an Amazon S3 bucket. The following parameters are used to configure this plugin:

* `access_key` - amazon key
* `secret_key` - amazon secret key
* `bucket` - bucket name
* `region` - bucket region (`us-east-1`, `eu-west-1`, etc)
* `acl` - access to files that are uploaded (`private`, `public-read`, etc)
* `source` - location of folder to sync
* `target` - target folder in your S3 bucket
* `delete` - deletes files in the target not found in the source
* `content_type` - override default mime-types to use this value
* `content_encoding` - override default content encoding header for files
* `cache_control` - override default cache control header for files
* `metadata` - set custom metadata
* `redirects` - targets that should redirect elsewhere
* `cloudfront_distribution_id` - (optional) the cloudfront distribution id to invalidate after syncing

The following is a sample S3 configuration in your .drone.yml file:

```yaml
publish:
  s3_sync:
    acl: public-read
    region: "us-east-1"
    bucket: "my-bucket.s3-website-us-east-1.amazonaws.com"
    access_key: "970d28f4dd477bc184fbd10b376de753"
    secret_key: "9c5785d3ece6a9cdefa42eb99b58986f9095ff1c"
    source: folder/to/archive
    target: /target/location
    delete: true
    cloudfront_distribution_id: "9c5785d3ece6a9cdefa4"
```

The `acl`, `content_type`, `cache_control`, and `content_encoding` parameters can be passed as a string value to apply to all files, or as a map to apply to a subset of files.

For example:

```yaml
publish:
  s3_sync:
    acl:
      "public/*": public-read
      "private/*": private
    content_type:
      ".svg": image/svg+xml
    content_encoding:
      ".js": gzip
      ".css": gzip
    cache_control: "public, max-age: 31536000"
    region: "us-east-1"
    bucket: "my-bucket.s3-website-us-east-1.amazonaws.com"
    access_key: "970d28f4dd477bc184fbd10b376de753"
    secret_key: "9c5785d3ece6a9cdefa42eb99b58986f9095ff1c"
    source: folder/to/archive
    target: /target/location
    delete: true
```

In the case of `acl` the key of the map is a glob. If there are no matches in your settings for a given file, the default is `"private"`.

The `content_type` field the key is an extension including the leading dot `.`. If you want to set a content type for files with no extension, set the key to the empty string `""`. If there are no matches for the `content_type` of any file, one will automatically be determined for you.

In the  `content_encoding` field the key is an extension including the leading dot `.`. If you want to set a encoding type for files with no extension, set the key
to th empty string `""`. If there are no matches for the `content_encoding` of a file, no content-encoding header will be added.

In the  `cache_control` field the key is an extension including the leading dot `.`. If you want to set cahce control for files with no extension, set the key
to th empty string `""`. If there are no matches for the `cache_control` of a file, no cache-control header will be added.

The `metadata` field can be set as either an object where the keys are the metadata headers:

```yaml
publish:
  s3_sync:
    acl: public-read
    region: "us-east-1"
    bucket: "my-bucket.s3-website-us-east-1.amazonaws.com"
    access_key: "970d28f4dd477bc184fbd10b376de753"
    secret_key: "9c5785d3ece6a9cdefa42eb99b58986f9095ff1c"
    source: folder/to/archive
    target: /target/location
    delete: true
    metadata:
      custom-meta: "abc123"
```

Or you can specify metadata for file patterns by using a glob:

```yaml
publish:
  s3_sync:
    acl: public-read
    region: "us-east-1"
    bucket: "my-bucket.s3-website-us-east-1.amazonaws.com"
    access_key: "970d28f4dd477bc184fbd10b376de753"
    secret_key: "9c5785d3ece6a9cdefa42eb99b58986f9095ff1c"
    source: folder/to/archive
    target: /target/location
    delete: true
    metadata:
      "*.png":
        CustomMeta: "abc123"
```

Additionally, you can specify redirect targets for files that don't exist by using the `redirects` key:

```yaml
publish:
  s3_sync:
    acl: public-read
    region: "us-east-1"
    bucket: "my-bucket.s3-website-us-east-1.amazonaws.com"
    access_key: "970d28f4dd477bc184fbd10b376de753"
    secret_key: "9c5785d3ece6a9cdefa42eb99b58986f9095ff1c"
    source: folder/to/archive
    target: /target/location
    delete: true
    redirects:
      some/missing/file: /somewhere/that/actually/exists
```
