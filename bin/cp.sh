#!/bin/bash
aws s3 cp $HOME/Downloads/login_plus_kingsin_s1_mod_2.rar s3://bucket1/submulti/login_plus_kingsin_s1_mod_2.rar --endpoint-url http://localhost:7080 --no-sign-request