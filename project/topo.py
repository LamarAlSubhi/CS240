#!/usr/bin/env python3
from mininet.net import Mininet
from mininet.topo import Topo
from mininet.link import TCLink
from mininet.node import OVSBridge
from mininet.cli import CLI


class GossipTopo(Topo):
    def build(self):
        s1 = self.addSwitch('s1', cls=OVSBridge)
        h1 = self.addHost('h1')
        h2 = self.addHost('h2')
        h3 = self.addHost('h3')

        # All symmetric for now
        self.addLink(h1, s1,cls=TCLink, bw=10, delay="10ms", loss=0)
        self.addLink(h2, s1, cls=TCLink, bw=10, delay="10ms", loss=0)
        self.addLink(h3, s1, cls=TCLink, bw=10, delay="10ms", loss=0)


def start_gossipd(net):
    gossip_bin = "/home/hassan/cs240/project/CS240/project/bin/linux/gossipd"

    hosts = [net.get(h) for h in ("h1", "h2", "h3")]
    ips = [h.IP() for h in hosts]

    for i, h in enumerate(hosts):
        node_id = i + 1
        ip = ips[i]

        bind = f"{ip}:9000"
        seeds = ",".join(f"{other_ip}:9000" for j, other_ip in enumerate(ips) if j != i)
        log_path = f"/tmp/gossip-{node_id}.jsonl"
        metrics_addr = "0.0.0.0:9080"

        cmd = (
            f"{gossip_bin} "
            f"-id={node_id} "
            f"-bind={bind} "
            f"-seeds={seeds} "
            f"-log={log_path} "
            f"-metrics-addr={metrics_addr} "
            f">/tmp/gossipd-{node_id}.log 2>&1 &"
        )

        print(f"Starting gossipd on {h.name}: {cmd}")
        h.cmd(cmd)


if __name__ == "__main__":
    topo = GossipTopo()
    net = Mininet(topo=topo, link=TCLink, controller=None, autoSetMacs=True)

    net.start()
    start_gossipd(net)

    CLI(net)

    net.stop()

