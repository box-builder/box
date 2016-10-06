box: a new type of builder for docker

build instructions:

* git clone https://github.com/erikh/box
* docker build -t box .
* docker run -v /var/run/docker.sock:/var/run/docker.sock -it box < demo.rb

Note that if you do not pass a filename over stdin, you will be prompted for
input where you can type a script in.
