from mininet.topo import Topo
from mininet.net import Mininet
from mininet.node import OVSController
from mininet.link import TCLink
from mininet.cli import CLI

class GossipTopo(Topo):
    def build(self):
        # 3 hosts
        h1 = self.addHost('h1')
        h2 = self.addHost('h2')
        h3 = self.addHost('h3')

        # Single switch
        s1 = self.addSwitch('s1')

        # Links
        self.addLink(h1, s1, cls=TCLink, delay='10ms')
        self.addLink(h2, s1, cls=TCLink, delay='10ms')
        self.addLink(h3, s1, cls=TCLink, delay='10ms')

if __name__ == '__main__':
    topo = GossipTopo()
    net = Mininet(topo=topo, controller=OVSController, link=TCLink)
    net.start()
    CLI(net)
    net.stop()