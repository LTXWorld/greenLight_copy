#!/bin/bash
set -eu

# ==================================================================================== #
# VARIABLES
# ==================================================================================== #
# Set the timezone for the server to China Standard Time.
TIMEZONE=Asia/Shanghai

# Set the name of the new user to create.
USERNAME=greenlight

# Prompt to enter a password for the PostgreSQL greenlight user (rather than hard-coding
# a password in this script).
read -p "Enter password for greenlight DB user: " DB_PASSWORD

# Force all output to be presented in en_US for the duration of this script. This avoids
# any "setting locale failed" errors while this script is running, before we have
# installed support for all locales. Do not change this setting!
export LC_ALL=en_US.UTF-8

# ==================================================================================== #
# SCRIPT LOGIC
# ==================================================================================== #

# Update all software packages.
yum update -y

# Set the system timezone.
timedatectl set-timezone ${TIMEZONE}

# Install necessary locales.
yum -y install glibc-common
localedef -i en_US -f UTF-8 en_US.UTF-8

# Add the new user and give them sudo privileges.
useradd --create-home --shell "/bin/bash" --groups wheel "${USERNAME}"

# Force a password to be set for the new user the first time they log in.
passwd --delete "${USERNAME}"
chage --lastday 0 "${USERNAME}"

# Copy the SSH keys from the root user to the new user.
rsync --archive --chown=${USERNAME}:${USERNAME} /root/.ssh /home/${USERNAME}

# Configure the firewall to allow SSH, HTTP and HTTPS traffic.
firewall-cmd --permanent --add-service=ssh
firewall-cmd --permanent --add-service=http
firewall-cmd --permanent --add-service=https
firewall-cmd --reload

# Install EPEL repository for additional packages.
yum install -y epel-release

# Install fail2ban.
yum install -y fail2ban
systemctl enable fail2ban
systemctl start fail2ban

# Install the migrate CLI tool.
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.14.1/migrate.linux-amd64.tar.gz | tar xvz
mv migrate.linux-amd64 /usr/local/bin/migrate

# Install PostgreSQL.
yum install -y postgresql-server postgresql-contrib
postgresql-setup initdb
systemctl enable postgresql
systemctl start postgresql

# Set up the greenlight DB and create a user account with the password entered earlier.
sudo -i -u postgres psql -c "CREATE DATABASE greenlight"
sudo -i -u postgres psql -d greenlight -c "CREATE EXTENSION IF NOT EXISTS citext"
sudo -i -u postgres psql -d greenlight -c "CREATE ROLE greenlight WITH LOGIN PASSWORD '${DB_PASSWORD}'"

# Add a DSN for connecting to the greenlight database to the system-wide environment
# variables in the /etc/environment file.
echo "GREENLIGHT_DB_DSN='postgres://greenlight:${DB_PASSWORD}@localhost/greenlight'" >> /etc/environment

# Install Caddy (see https://caddyserver.com/docs/install#fedora-rhel-centos).
yum install -y yum-plugin-copr
yum copr enable @caddy/caddy -y
yum install -y caddy

# Enable and start Caddy.
systemctl enable caddy
systemctl start caddy

# Reboot the server to apply all changes.
echo "Script complete! Rebooting..."
reboot
