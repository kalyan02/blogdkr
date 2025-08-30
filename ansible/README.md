# Server Provisioning with Ansible

Two-mode Ansible system for Ubuntu/Debian servers: **provision** (initial setup) and **prod** (ongoing maintenance).

## Two-Mode System

### 1. Provision Mode (`make provision`)
- **Run ONCE** on fresh servers
- Connects via port 22 as root
- Changes SSH port to 2222
- Creates kalyan user, disables root
- **Not idempotent** (port change breaks connectivity)

### 2. Production Mode (`make prod`) 
- **Run MULTIPLE times** safely
- Connects via port 2222 as kalyan
- **Fully idempotent** - safe to run repeatedly
- Includes system updates, maintenance, monitoring

## Features Applied

- **User Management**: kalyan user with sudo, SSH keys
- **Docker**: Docker CE + docker-compose installed
- **SSH Hardening**: Port 2222, key-only auth, no root
- **Firewall**: UFW (80, 443, 2222 open)
- **Fail2ban**: SSH brute force protection  
- **Swap**: 2GB swap with optimal settings
- **Security**: Kernel hardening, process accounting
- **Maintenance**: Package updates, cleanup, monitoring

## Quick Start

### 1. Setup
```bash
# Edit server details
vim ansible/hosts.yml

# Test connectivity (fresh server)
make ping
```

### 2. Initial Provisioning
```bash
# One-time setup (fresh server on port 22)
make provision
```

### 3. Ongoing Maintenance  
```bash
# Regular maintenance (provisioned server on port 2222)
make prod
```

## Security Features Applied

### Network Security
- UFW firewall (deny by default, allow 80/443/2222)
- Fail2ban monitoring SSH attempts
- Disabled unused network protocols (DCCP, SCTP, RDS, TIPC)
- SYN flood protection via kernel parameters

### SSH Security
- Port changed to 2222
- Root login disabled
- Password authentication disabled
- Only key-based authentication
- Connection limits and timeouts
- Limited to `kalyan` user only

### System Security
- Kernel hardening parameters
- Secure shared memory mounting
- Process accounting enabled
- Security monitoring tools (rkhunter, chkrootkit, lynis)
- Automatic security updates
- Root account locked and disabled

### Additional Hardening Recommendations

Consider implementing:
- **Log monitoring**: ELK stack or similar
- **Intrusion detection**: AIDE file integrity monitoring
- **Network monitoring**: ntopng or similar
- **Backup monitoring**: Automated backup verification
- **Certificate management**: Let's Encrypt automation
- **Container security**: Docker bench, Falco
- **Vulnerability scanning**: OpenVAS or similar

## File Structure

```
ansible/
├── provision.yml           # Main playbook
├── hosts.yml              # Inventory file
├── roles/
│   ├── common/tasks/
│   │   ├── system_update.yml
│   │   ├── user_setup.yml
│   │   ├── swap.yml
│   │   ├── sshd.yml
│   │   ├── firewall.yml
│   │   ├── fail2ban.yml
│   │   └── root_cleanup.yml
│   ├── docker/tasks/
│   │   └── main.yml
│   └── security/tasks/
│       └── main.yml
```

## Variables

Edit variables in `provision.yml`:
- `the_user`: Username to create (default: kalyan)
- `ssh_port`: SSH port (default: 2222) 
- `swap_file_size`: Swap file size (default: 2G)

## Port Transition

1. **Initial**: Server accessible on port 22 as root
2. **After provisioning**: Server accessible on port 2222 as kalyan
3. **Update inventory**: Change `ansible_port` from 22 to 2222 for future runs

## Troubleshooting

**Connection refused after provisioning**:
- Ensure using port 2222: `ssh -p 2222 kalyan@server`
- Check UFW status: `sudo ufw status`
- Verify SSH config: `sudo systemctl status sshd`

**Docker permission denied**:
- Log out and back in for docker group membership
- Or run: `sudo usermod -aG docker kalyan && newgrp docker`