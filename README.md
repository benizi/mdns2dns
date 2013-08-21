# mdns2dns

DNS server written in Go that serves .local addresses under .4m.

# Current state

Currently, it doesn't do anything with .local names.  It just lets you:

1. Register your name at whatever.in.4m

2. Query registered names as (any.leading.parts.)whatever.4m


## Example

```sh
$ dig +short newname.in.4m
$ dig +short aardmark.newname.4m
192.168.30.19 # should return a non-loopback IP of the current machine
```

# Utility

If you set Pow up to handle the domain `newname.4m`, you can serve your apps as
`appname.newname.4m`.  Per the [Pow User's
Manual](http://pow.cx/manual.html#section_3.1), set your POW_EXT_DOMAINS
variable to `newname.4m`.

# Disclaimer

This is mainly an excuse for me to play with Go.
