#!/usr/bin/env python3

from mininet.topo import Topo
from mininet.net import Mininet
from mininet.node import OVSBridge
from mininet.link import TCLink
from mininet.cli import CLI

class GossipTopo(Topo):
    def build(self):
        h1 = self.addHost('h1')
        h2 = self.addHost('h2')
        h3 = self.addHost('h3')

        # Force OVSBridge (learning switch, no OpenFlow)
        s1 = self.addSwitch('s1', cls=OVSBridge)

        self.addLink(h1, s1, delay='10ms')
        self.addLink(h2, s1, delay='10ms')
        self.addLink(h3, s1, delay='10ms')

if __name__ == '__main__':
    topo = GossipTopo()

    # Force switch type to OVSBridge and remove controller entirely
    net = Mininet(
        topo=topo,
        controller=None,
        switch=OVSBridge,
        link=TCLink
    )

    net.start()
    CLI(net)
    net.stop()
