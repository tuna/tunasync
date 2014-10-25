#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sys
import socket
import argparse
import json

if __name__ == "__main__":
    parser = argparse.ArgumentParser(prog="tunasynctl")
    parser.add_argument("-s", "--socket",
                        default="/var/run/tunasync.sock", help="socket file")
    parser.add_argument("command", help="command")
    parser.add_argument("target", help="target")

    args = parser.parse_args()

    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)

    try:
        sock.connect(args.socket)
    except socket.error as msg:
        print(msg)
        sys.exit(1)

    pack = json.dumps({
        'cmd': args.command,
        'target': args.target,
    })

    try:
        sock.sendall(chr(len(pack)) + pack)
        length = ord(sock.recv(1))
        print(sock.recv(length))

    except Exception as e:
        print(e)
    finally:
        sock.close()

# vim: ts=4 sw=4 sts=4 expandtab
