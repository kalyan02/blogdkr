.PHONY: help provision prod ping check

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Main Commands:'
	@echo '  make provision  - Initial server setup (run ONCE on fresh server)'
	@echo '  make prod       - Production maintenance (run multiple times)'
	@echo ''
	@echo 'Other Commands:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

ping: ## Test connectivity to servers
	@echo "Testing provision servers (port 22)..."
	ansible -i hosts.yml provision -m ping
	@echo "Testing production servers (port 2222)..."
	ansible -i hosts.yml production -m ping

provision: ## Initial server provisioning (fresh server, port 22 -> 2222)
	@echo "=== INITIAL SERVER PROVISIONING ==="
	@echo "This will:"
	@echo "- Create kalyan user with sudo access"
	@echo "- Install Docker and docker-compose" 
	@echo "- Change SSH port from 22 to 2222"
	@echo "- Configure firewall (UFW)"
	@echo "- Setup fail2ban"
	@echo "- Disable root access"
	@echo ""
	@read -p "Continue with provisioning? [y/N] " confirm && [ "$$confirm" = "y" ]
	ansible-playbook -i hosts.yml provision.yml

prod: ## Production maintenance (idempotent, can run multiple times)
	@echo "=== PRODUCTION MAINTENANCE ==="
	@echo "Running maintenance tasks..."
	ansible-playbook -i hosts.yml prod.yml

check: ## Dry run - show what would be changed
	@echo "=== DRY RUN - PROVISION ==="
	ansible-playbook -i hosts.yml provision.yml --check --diff
	@echo ""
	@echo "=== DRY RUN - PRODUCTION ==="
	ansible-playbook -i hosts.yml prod.yml --check --diff

facts: ## Gather system information
	ansible -i hosts.yml production -m setup

status: ## Show server status
	@echo "=== SERVER STATUS ==="
	ansible -i hosts.yml production -m command -a "uptime"
	@echo ""
	@echo "=== DISK SPACE ==="
	ansible -i hosts.yml production -m command -a "df -h"
	@echo ""
	@echo "=== MEMORY ==="
	ansible -i hosts.yml production -m command -a "free -h"
	@echo ""
	@echo "=== DOCKER STATUS ==="
	ansible -i hosts.yml production -m command -a "systemctl is-active docker"

logs: ## Show security logs
	@echo "=== FAIL2BAN STATUS ==="
	ansible -i hosts.yml production -m command -a "sudo fail2ban-client status" --become
	@echo ""
	@echo "=== UFW STATUS ==="
	ansible -i hosts.yml production -m command -a "sudo ufw status" --become
	@echo ""
	@echo "=== RECENT FAILED LOGINS ==="
	ansible -i hosts.yml production -m command -a "sudo grep 'Failed password' /var/log/auth.log | tail -5" --become

ssh: ## SSH to production server using hosts.yml config
	@echo "Getting SSH config from hosts.yml..."
	@ansible-inventory -i hosts.yml --host prod-server > /tmp/ansible_host.json
	@python3 -c "import json, os; data=json.load(open('/tmp/ansible_host.json')); cmd=f\"ssh -A -p {data['ansible_port']} -i {data['ansible_ssh_private_key_file']} {data['ansible_ssh_extra_args']} {data['ansible_user']}@{data['ansible_host']}\"; print(f'Connecting: {cmd}'); os.system(cmd)"
	@rm -f /tmp/ansible_host.json