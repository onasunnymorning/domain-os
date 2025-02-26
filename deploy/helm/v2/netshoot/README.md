# create an ephemeral container for network troubleshooting

Ref. https://github.com/nicolaka/netshoot?tab=readme-ov-file#netshoot-with-kubernetes

`kubectl run tmp-shell --rm -i --tty --image nicolaka/netshoot`
