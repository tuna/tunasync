<li>删除某worker的某镜像

先确定已经给tunasynctl写好config文件：<code>~/.config/tunasync/ctl.conf</code>
<pre><code>manager_addr = "127.0.0.1"
manager_port = 12345
ca_cert = ""</code></pre>


接着
<pre><code>$ tunasynctl disable -w [worker_id] [mirror_name]
$ tunasynctl flush</code></pre>

<li>热重载<code>worker.conf</code>

<code>$ tunasynctl reload -w [worker_id]</code>

----

e.g. 删除<code>test_worker</code>的<code>elvish</code>镜像：

1. 删除存放镜像的文件夹

1. 删除<code>worker.conf</code>中对应的mirror段落

1. 接着操作：
<pre><code>$ tunasynctl reload -w test_worker
$ tunasynctl disable -w test_worker elvish
$ tunasynctl flush</code></pre>

1. （可选）最后删除日志文件夹里的日志
----

<li>删除worker

<code>$ tunasynctl rm-worker -w [worker_id]</code>

e.g. <code>$ tunasynctl rm-worker -w test_worker</code>
