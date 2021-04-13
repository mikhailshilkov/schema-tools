# schema-tools

Tools to analyze Pulumi schemas.

## Building

```
go install
```

## Resource Stats

```
$ schema-tools stats azure-native
Provider: azure-native
Total resources: 1056
Unique resources: 1056
Total properties: 13018
```

## Schema Comparison

```
$ schema-tools compare aws master 4379b20d1aab018bac69c6d86c4219b08f8d3ec4                      
Found 1 breaking change:
Function "aws:s3/getBucketObject:getBucketObject" missing input "bucketKeyEnabled"
```
