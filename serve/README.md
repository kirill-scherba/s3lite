# HTTP server for S3 like native Golang storage system

## Command line interface to check server handlers

### Get list of buckets

```bash
aws s3 ls --endpoint-url http://localhost:7080 --no-sign-request

curl 'http://localhost:7080?pretty=true'
```

### Get list of objects in bucket

```bash
aws s3 ls s3://bucket1 --endpoint-url http://localhost:7080 --no-sign-request

# curl http://localhost:7080/bucket1/objects
curl 'http://localhost:7080/bucket1?list-type=2&recursive=true&pretty=true'
curl 'http://localhost:7080/bucket1?list-type=2&prefix=sub1&recursive=true&pretty=true'


# Recursive

aws s3 ls s3://bucket1 --recursive --endpoint-url http://localhost:7080 --no-sign-request

curl 'http://localhost:7080/bucket1?list-type=2&pretty=true&delimiter=/'
curl 'http://localhost:7080/bucket1?list-type=2&prefix=sub1&recursive=true&pretty=true&delimiter=/'
```

### Get object

```bash
aws s3 cp s3://bucket1/key1 downloaded.txt --endpoint-url http://localhost:7080 --no-sign-request

curl http://localhost:7080/bucket1/key1
```

### Put object

```bash
aws s3 cp downloaded.txt s3://bucket1/key1 --endpoint-url http://localhost:7080 --no-sign-request

curl -X PUT -T downloaded.txt http://localhost:7080/bucket1/key3
```

## Licence

BSD
