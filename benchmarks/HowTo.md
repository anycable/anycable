## Hot to run benchmarks

### Installation

- Install [Ansible](http://ansible.com) (`brew install ansible`)

- Launch two EC2 instances from `anycable-benchmark-xxx` AMI

- Attach more local network interfaces to the _client_ instance

- Update `hosts` file and playbooks with required information (IPs, etc)

- Prepare the _client_ instance: `ansible-playbook --tags prepare benchmark.yml`

### Running benchmarks

To run a server, e.g. Action Cable: `ansible-playbook --tags action_cable servers.yml`

To run a benchmark against it: `ansible-playbook --tags action_cable benchmark.yml`

You can also specify benchmark parameters: `ansible-playbook --extra-vars "step_size=1000 steps=10 sample_size=10" benchmark.yml`.

**NOTE**: Ansible doesn't support command output streaming, so we can only see the results at the end of the run.
