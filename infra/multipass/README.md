using this as a base
https://blog.kubesimplify.com/kubernetes-on-apple-macbooks-m-series#heading-provisioning-the-controlplane-instance-kubemaster

going to deploy just one larger node

multipass launch --disk 10G --memory 7G --cpus 4 --name kmaster --network name=en0,mode=manual,mac="52:54:00:4b:ab:cd" noble


the ip address is: 192.168.64.11

multipass exec -n kmaster -- sudo bash -c 'cat << EOF > /etc/netplan/10-custom.yaml
network:
  version: 2
  ethernets:
    extra0:
      dhcp4: no
      match:
        macaddress: "52:54:00:4b:ab:cd"
      addresses: [192.168.64.101/24]
EOF'

multipass exec -n kmaster -- sudo netplan apply

▶ multipass info kmaster | grep IPv4 -A1
IPv4:           192.168.64.11
                192.168.64.101


multipass shell kmaster => get shell 

install k3s

ubuntu@kmaster:~$ sudo ufw disable
Firewall stopped and disabled on system startup

add a mount from the host

▶ multipass mount /Users/gprins kmaster

install k3s

ubuntu@kmaster:~$ curl -sfL https://get.k3s.io | sh -
[INFO]  Finding release for channel stable
[INFO]  Using v1.30.5+k3s1 as release
[INFO]  Downloading hash https://github.com/k3s-io/k3s/releases/download/v1.30.5+k3s1/sha256sum-arm64.txt
[INFO]  Downloading binary https://github.com/k3s-io/k3s/releases/download/v1.30.5+k3s1/k3s-arm64
[INFO]  Verifying binary download
[INFO]  Installing k3s to /usr/local/bin/k3s
[INFO]  Skipping installation of SELinux RPM
[INFO]  Creating /usr/local/bin/kubectl symlink to k3s
[INFO]  Creating /usr/local/bin/crictl symlink to k3s
[INFO]  Creating /usr/local/bin/ctr symlink to k3s
[INFO]  Creating killall script /usr/local/bin/k3s-killall.sh
[INFO]  Creating uninstall script /usr/local/bin/k3s-uninstall.sh
[INFO]  env: Creating environment file /etc/systemd/system/k3s.service.env
[INFO]  systemd: Creating service file /etc/systemd/system/k3s.service
[INFO]  systemd: Enabling k3s unit
Created symlink /etc/systemd/system/multi-user.target.wants/k3s.service → /etc/systemd/system/k3s.service.
[INFO]  systemd: Starting k3s


