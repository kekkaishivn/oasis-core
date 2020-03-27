#!/usr/bin/env python
"""
Starts Prometheus docker container and setups docker networking so
Oasis node and Prometheus containers can be connected.

Script assumes Oasis node container is already running and by default will
attach to container named "oasis-node-*****". Use "--oasis-node-name" to
override the default name.

Usage:
    ./scripts/prometheus.py \
      --config "path/to/prometheus.yml"

For help, see:
    ./scripts/prometheus.py -h
"""

import argparse
import subprocess
import sys
import os
import json
import atexit
import signal


PROMETHEUS_IMAGE_TAG = "quay.io/prometheus/prometheus:latest"
PROMETHEUS_PUSHGATEWAY_IMAGE_TAG = "prom/pushgateway:latest"


def get_container_id(name):
    output = subprocess.check_output(
        # Prefix match.
        ['docker', 'ps', '-q', '-f', 'name=^/{}'.format(name)]
    )
    return output.split('\n')[0]


def container_running(name):
    container_id = get_container_id(name)
    return not container_id == ''


def network_exists(name):
    docker_network = subprocess.Popen(
        ['docker', 'network', 'ls'], stdout=subprocess.PIPE)
    try:
        output = subprocess.check_output(
            ['grep', name], stdin=docker_network.stdout)
        docker_network.wait()
    except subprocess.CalledProcessError:
        # No grep match
        return False
    return not output.split('\n')[0] == ''


def create_network(name):
    subprocess.check_call(
        ['docker', 'network', 'create', name]
    )


def connect_container(network_name, container_name):
    """ Connects container to network *network_name*. """

    container_id = get_container_id(container_name)
    # Check if container allready connected to the network:
    output = subprocess.check_output(
        ['docker', 'network', 'inspect', network_name]
    )
    parsed = json.loads(output)[0]
    for container in parsed['Containers']:
        if container.startswith(container_id):
            return True

    # Container not yet connected.
    subprocess.check_call(
        ['docker', 'network', 'connect', '--alias',
            'oasis-node', network_name, container_id]
    )


def cleanup_container(container_name):
    """ Removes exited container with *container_name* name. """

    command = ['docker', 'ps', '-aq', '-f', 'status=exited',
               '-f', 'name={}'.format(container_name)]
    output = subprocess.check_output(command).split('\n')[0]
    if output != '':
        subprocess.check_call(['docker', 'rm', output])


def run_prometheus(container_name, network_name,
                   exposed_port, prometheus_config_path):
    cleanup_container(container_name)
    command = ['docker', 'run', '--name', container_name, '--network={}'.format(network_name), '--network-alias', 'prometheus', '-p',
               '{}:9090'.format(exposed_port), "-v", "{}:/etc/prometheus/prometheus.yml".format(prometheus_config_path), PROMETHEUS_IMAGE_TAG]
    subprocess.check_call(command)


def run_pushgateway(container_name, network_name):
    cleanup_container(container_name)
    # Starts pushgateway in background.
    if not container_running(container_name):
        command = ['docker', 'run', '--name', container_name, '--network={}'.format(network_name), '--network-alias', 'pushgateway',
                   PROMETHEUS_PUSHGATEWAY_IMAGE_TAG]
        pid = subprocess.Popen(command).pid

        atexit.register(lambda: os.kill(pid, signal.SIGTERM))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=None)
    optional = parser._action_groups.pop()
    required = parser.add_argument_group('required arguments')

    required.add_argument('--config', type=str, required=True,
                          help="Path to prometheus.yml config.")
    optional.add_argument('--oasis-node-name', type=str, default="oasis-node-",
                          help="Running oasis node container name.")
    optional.add_argument('--name', type=str, default="prometheus",
                          help="Prometheus container name. Default: prometheus")
    optional.add_argument('--network-name', type=str, default="prometheus-oasis-node",
                          help="Network name for prometheus-oasis-node container connection. Default: prometheus-oasis-node")
    optional.add_argument('--port', type=str, default='9090',
                          help="Localhost port exposing prometheus web interface. Default: 9090")
    optional.add_argument('--push-gateway', action='store_true', default=False,
                          help="If present, starts push-gateway instance as well.")

    parser._action_groups.append(optional)
    args = parser.parse_args()

    network_name = args.network_name
    oasis_node_container = args.oasis_node_name
    prometheus_container = args.name
    exposed_port = args.port
    prometheus_config = args.config

    push_gateway = args.push_gateway

    if not os.path.isfile(prometheus_config):
        print("ERROR: Prometheus config file not found: '{}'".format(
            prometheus_config))
        sys.exit(1)

    prometheus_config = os.path.abspath(prometheus_config)

    # Checked if passed Oasis node container is running
    if not container_running(oasis_node_container):
        print("ERROR: Oasis node container: '{}' not running!".format(oasis_node_container))
        sys.exit(1)

    if not network_exists(network_name):
        create_network(network_name)

    # Connect Oasis node container to network (if not yet connected).
    connect_container(network_name, oasis_node_container)

    if push_gateway:
        run_pushgateway("prometheus-pushgateway", network_name)

    # Check if prometheus is allready running
    if not container_running(prometheus_container):
        # If not: run it (& clean any old exited prometheus containers)
        run_prometheus(prometheus_container, network_name,
                       exposed_port, prometheus_config)
    else:
        print("ERROR: Prometheus container: '{}, allready running!".format(
            prometheus_container))
