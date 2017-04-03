## Getting Box

**[Download a Release](https://github.com/erikh/box/releases/)**

## Installation

Just `gunzip` the downloaded file and put it in your path:

```bash
$ gunzip box.$(uname -s).gz
$ chmod 755 box.$(uname -s)
$ sudo mv box.$(uname -s) /usr/local/bin/box
```

Alternatively, we have a [homebrew tap](https://github.com/erikh/homebrew-box)
and debian and redhat packages on the [releases page](https://github.com/erikh/box/releases).

## Invocation

### Use the shell

If you want to try out box quickly, you can use the shell interface, AKA
repl (read-eval-print loop):


```bash
$ box repl
# or
$ box shell
```

This video gives a quick demo of the shell:

<script type="text/javascript" src="https://asciinema.org/a/c1n0h0g73f10x4cuzjf1i51vg.js" id="asciicast-c1n0h0g73f10x4cuzjf1i51vg" async></script>