#!/bin/bash
export AWS_ACCESS_KEY_ID=ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=SECRET_ACCESS_KEY
aws s3 ls s3://bucket1/submulti --endpoint-url http://localhost:7080

# aws s3 ls s3://bucket1/submulti --endpoint-url http://localhost:7080 --no-sign-request
# echo "ACCESS_KEY:SECRET_ACCESS_KEY" > ~/.passwd-s3fs
# chmod 600 ~/.passwd-s3fs
# sudo s3fs bucket1 /mnt/s3lite -o passwd_file=~/.passwd-s3fs -o url=http://localhost:7080 -o use_path_request_style -o no_check_certificate