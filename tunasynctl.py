#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sys
import socket
import argparse
import json
import struct

if __name__ == "__main__":
    parser = argparse.ArgumentParser(prog="tunasynctl")
    parser.add_argument("-s", "--socket",
                        default="/var/run/tunasync.sock", help="socket file")

    subparsers = parser.add_subparsers(dest="command", help='sub-command help')

    sp = subparsers.add_parser('start', help="start job")
    sp.add_argument("target", help="mirror job name")

    sp = subparsers.add_parser('stop', help="stop job")
    sp.add_argument("target", help="mirror job name")

    sp = subparsers.add_parser('restart', help="restart job")
    sp.add_argument("target", help="mirror job name")

    sp = subparsers.add_parser('status', help="show mirror status")
    sp.add_argument("target", nargs="?", default="__ALL__", help="mirror job name")

    sp = subparsers.add_parser('log', help="return log file path")
    sp.add_argument("-n", type=int, default=0, help="last n-th log, default 0 (latest)")
    sp.add_argument("target", help="mirror job name")

    sp = subparsers.add_parser('help', help="show help message")

    args = vars(parser.parse_args())

    if args['command'] == "help":
        parser.print_help()
        sys.exit(0)

    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)

    try:
        sock.connect(args.pop("socket"))
    except socket.error as msg:
        print(msg)
        sys.exit(1)

    pack = json.dumps({
        "cmd": args.pop("command"),
        "target": args.pop("target"),
        "kwargs": args,
    })

    try:
        sock.sendall(struct.pack('!H', len(pack)) + pack)
        length = struct.unpack('!H', sock.recv(2))[0]
        print(sock.recv(length))

    except Exception as e:
        print(e)
    finally:
        sock.close()

# vim: ts=4 sw=4 sts=4 expandtab
