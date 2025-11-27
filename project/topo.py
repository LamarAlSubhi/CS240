#!/usr/bin/env python3
# ===============================
# Imports
# ===============================

import argparse
import os
import sys
from mininet.net import Mininet
from mininet.topo import Topo
from mininet.link import TCLink
from mininet.node import OVSBridge
from mininet.cli import CLI


# ===============================
# Topology Definition
# ===============================

class GossipTopo(Topo):
    """
    Creates N hosts connected to a single OVSBridge switch.
    Each host gets identical link characteristics.
    """

    def build(self, n_hosts, bw, delay, loss):
        s1 = self.addSwitch("s1", cls=OVSBridge)
        self.hosts_list = []

        for i in range(1, n_hosts + 1):
            host = self.addHost(f"h{i}")

            # Add symmetric link with specified characteristics
            self.addLink(
                host, s1,
                cls=TCLink,
                bw=bw,
                delay=delay,
                loss=loss
            )

            self.hosts_list.append(host)


# ===============================
# Gossip Daemon Launcher
# ===============================

def start_gossipd(net, gossip_bin, fanout, ttl, period, metrics_port, log_dir):
    """
    Launch gossipd on each Mininet host with the configured arguments.

    Each host gets:
      - A unique id (1..N)
      - Bind address <IP>:9000
      - A seed list containing all other hosts
      - A per-host log file in log_dir
      - Shared metrics server port (HTTP)
    """
    hosts = net.hosts

    for idx, host in enumerate(hosts, start=1):
        ip = host.IP()
        bind_addr = f"{ip}:9000"

        # Construct peer list (all other hosts)
        seeds = [
            f"{other.IP()}:9000"
            for other in hosts
            if other != host
        ]
        seed_str = ",".join(seeds)

        log_path = os.path.join(log_dir, f"gossip-{idx}.jsonl")
        out_path = os.path.join(log_dir, f"gossipd-{idx}.out")
        metrics_addr = f"0.0.0.0:{metrics_port}"

        # Build command string for gossipd
        cmd = (
            f"{gossip_bin} "
            f"-id={idx} "
            f"-bind={bind_addr} "
            f"-seeds={seed_str} "
            f"-fanout={fanout} "
            f"-ttl={ttl} "
            f"-period={period} "
            f"-metrics-addr={metrics_addr} "
            f"-log={log_path} "
            f"> {out_path} 2>&1 &"
        )

        print(f"[INFO] Launching gossipd on {host.name}: {cmd}")
        host.cmd(cmd)


# ===============================
# Argument Parser
# ===============================

def parse_args():
    """Defines all CLI arguments used for experimentation."""
    parser = argparse.ArgumentParser(
        description="Run gossip protocol over a Mininet topology."
    )

    # Location of gossip binary
    parser.add_argument("--bin", required=True,
                        help="Path to gossipd binary (relative or absolute).")

    # Network topology parameters
    parser.add_argument("--hosts", type=int, default=3,
                        help="Number of gossip hosts.")
    parser.add_argument("--bw", type=float, default=10,
                        help="Bandwidth in Mbps.")
    parser.add_argument("--delay", default="10ms",
                        help="Propagation delay.")
    parser.add_argument("--loss", type=float, default=0,
                        help="Packet loss percentage.")

    # Gossip protocol parameters
    parser.add_argument("--fanout", type=int, default=3)
    parser.add_argument("--ttl", type=int, default=8)
    parser.add_argument("--period", default="200ms")

    # Logging and metrics
    parser.add_argument("--metrics-port", type=int, default=9080)
    parser.add_argument("--logdir", default="/tmp",
                        help="Directory for event logs and stdout.")

    return parser.parse_args()


# ===============================
# Main Entry Point
# ===============================

def main():
    args = parse_args()

    # Validate gossip binary
    if not (os.path.isfile(args.bin) and os.access(args.bin, os.X_OK)):
        print(f"ERROR: gossipd not executable at: {args.bin}", file=sys.stderr)
        sys.exit(1)

    # Build topology the correct Mininet way
    topo = GossipTopo(
        n_hosts=args.hosts,
        bw=args.bw,
        delay=args.delay,
        loss=args.loss,
    )

    net = Mininet(
        topo=topo,
        link=TCLink,
        controller=None,
        autoSetMacs=True
    )
    net.start()

    # Launch gossip processes
    start_gossipd(
        net,
        gossip_bin=args.bin,
        fanout=args.fanout,
        ttl=args.ttl,
        period=args.period,
        metrics_port=args.metrics_port,
        log_dir=args.logdir
    )

    CLI(net)
    net.stop()



if __name__ == "__main__":
    main()

