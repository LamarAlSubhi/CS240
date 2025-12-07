#!/usr/bin/env python3
import argparse
import os
import sys
import time
from mininet.net import Mininet
from mininet.topo import Topo
from mininet.link import TCLink
from mininet.node import OVSBridge
from mininet.cli import CLI


# ======================================================
# Topology
# ======================================================

class GossipTopo(Topo):
    """
    Creates N hosts connected to a single OVSBridge switch.
    Each host gets identical link characteristics.
    """
    def build(self, n_hosts, bw, delay, loss, jitter="0ms",
              reorder=0, corrupt=0, duplicate=0):

        s1 = self.addSwitch("s1", cls=OVSBridge)
        self.hosts_list = []

        for i in range(1, n_hosts + 1):
            host = self.addHost(f"h{i}")

            self.addLink(
                host, s1,
                cls=TCLink,
                bw=bw,
                delay=delay,
                loss=loss,
                jitter=jitter,
                reorder=reorder,
                corrupt=corrupt,
                duplicate=duplicate
            )

            self.hosts_list.append(host)


# ======================================================
# Launch gossip daemon
# ======================================================

def start_gossipd(net, gossip_bin, fanout, ttl, period, metrics_port,
                  log_dir, zombie=None):

    hosts = net.hosts

    for idx, host in enumerate(hosts, start=1):
        ip = host.IP()
        bind_addr = f"{ip}:9000"

        # Make one host "zombie" — fanout = 0 so it never sends
        host_fanout = 0 if (zombie == idx) else fanout

        seeds = [
            f"{other.IP()}:9000"
            for other in hosts
            if other != host
        ]
        seed_str = ",".join(seeds)

        log_path = os.path.join(log_dir, f"gossip-{idx}.jsonl")
        out_path = os.path.join(log_dir, f"gossipd-{idx}.out")
        metrics_addr = f"0.0.0.0:{metrics_port}"

        cmd = (
            f"{gossip_bin} "
            f"-id={idx} "
            f"-bind={bind_addr} "
            f"-seeds={seed_str} "
            f"-fanout={host_fanout} "
            f"-ttl={ttl} "
            f"-period={period} "
            f"-metrics-addr={metrics_addr} "
            f"-log={log_path} "
            f"> {out_path} 2>&1 &"
        )

        print(f"[INFO] Launching gossipd on {host.name}: fanout={host_fanout}")
        host.cmd(cmd)


# ======================================================
# Args
# ======================================================

def parse_args():
    parser = argparse.ArgumentParser()

    parser.add_argument("--bin", required=True)
    parser.add_argument("--hosts", type=int, default=3)
    parser.add_argument("--bw", type=float, default=10)
    parser.add_argument("--delay", default="10ms")
    parser.add_argument("--loss", type=float, default=0)

    # NEW network impairments
    parser.add_argument("--jitter", default="0ms")
    parser.add_argument("--reorder", type=float, default=0)
    parser.add_argument("--corrupt", type=float, default=0)
    parser.add_argument("--duplicate", type=float, default=0)

    parser.add_argument("--fanout", type=int, default=3)
    parser.add_argument("--ttl", type=int, default=8)
    parser.add_argument("--period", default="200ms")
    parser.add_argument("--metrics-port", type=int, default=9080)
    parser.add_argument("--logdir", default="/tmp")

    parser.add_argument("--no-cli", action="store_true")

    parser.add_argument("--inject-rumor")
    parser.add_argument("--inject-ttl", type=int, default=8)
    parser.add_argument("--runtime", type=float, default=2.0)

    # NEW
    parser.add_argument("--zombie", type=int, default=None,
                        help="Host index that should never send gossip")
    parser.add_argument("--churn", type=int, default=None,
                        help="Kill and revive k hosts mid-run (not implemented yet)")

    return parser.parse_args()


# ======================================================
# Main
# ======================================================

def main():
    args = parse_args()

    if not (os.path.isfile(args.bin) and os.access(args.bin, os.X_OK)):
        print(f"ERROR: gossipd not executable: {args.bin}")
        sys.exit(1)

    topo = GossipTopo(
        n_hosts=args.hosts,
        bw=args.bw,
        delay=args.delay,
        loss=args.loss,
        jitter=args.jitter,
        reorder=args.reorder,
        corrupt=args.corrupt,
        duplicate=args.duplicate,
    )

    net = Mininet(
        topo=topo,
        link=TCLink,
        controller=None,
        autoSetMacs=True
    )

    net.start()

    start_gossipd(
        net,
        gossip_bin=args.bin,
        fanout=args.fanout,
        ttl=args.ttl,
        period=args.period,
        metrics_port=args.metrics_port,
        log_dir=args.logdir,
        zombie=args.zombie
    )
    time.sleep(1.5)

    time.sleep(1.0)

    # Automated mode
    if args.no_cli:
        if args.inject_rumor:
            h1 = net.hosts[0]
            curl_cmd = (
                f'curl "http://127.0.0.1:{args.metrics_port}/inject'
                f'?id={args.inject_rumor}&ttl={args.inject_ttl}"'
            )
            print("[INFO] Injecting rumor...")
            out = h1.cmd(curl_cmd)
            print("[INFO] Inject result:", out.strip())

        if args.runtime > 0:
            print(f"[INFO] Sleeping {args.runtime} seconds")
            time.sleep(args.runtime)

        net.stop()
        return

    CLI(net)
    net.stop()


if __name__ == "__main__":
    main()
