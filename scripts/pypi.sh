#!/bin/bash
if [ ! -d "$TUNASYNC_WORKING_DIR" ]; then
	echo "Directory not exists, fail"
	exit 1	
fi

/usr/bin/timeout -s INT 3600 /home/tuna/.virtualenvs/bandersnatch/bin/bandersnatch -c /etc/bandersnatch.conf mirror || exit 1
