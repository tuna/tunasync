#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import socket
import os
import json


class ControlServer(object):

    def __init__(self, address, mgr_chan, cld_chan):
        self.address = address
        self.mgr_chan = mgr_chan
        self.cld_chan = cld_chan
        try:
            os.unlink(self.address)
        except OSError:
            if os.path.exists(self.address):
                raise Exception("file exists: {}".format(self.address))

        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.bind(self.address)

        print("Control Server listening on: {}".format(self.address))
        self.sock.listen(1)

    def serve_forever(self):
        while 1:
            conn, _ = self.sock.accept()

            try:
                length = ord(conn.recv(1))
                content = conn.recv(length)
                cmd = json.loads(content)
                self.mgr_chan.put(("CMD", (cmd['cmd'], cmd['target'])))
            except Exception as e:
                print(e)
                res = "Invalid Command"
            else:
                res = self.cld_chan.get()

            conn.sendall(chr(len(res)) + res)
            conn.close()


def run_control_server(address, mgr_chan, cld_chan):
    cs = ControlServer(address, mgr_chan, cld_chan)
    cs.serve_forever()

# vim: ts=4 sw=4 sts=4 expandtab
