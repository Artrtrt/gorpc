GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "-s -w"
chmod a+x client
# scp ./private.pem root@192.168.1.1:/tmp/
# scp ./public.pem root@192.168.1.1:/tmp/
scp ./client root@192.168.1.1:/tmp/