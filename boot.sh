#! /bin/bash

log_dir="./logs"
if [ ! -d "$log_dir" ]; then
	mkdir logs
fi

exec ./wiseLog -alsologtostderr=true -log_dir=./logs -v=2
