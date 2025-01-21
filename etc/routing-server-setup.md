# keygen
ssh-keygen -t ed25519 -C "email"

# REDIS SETUP https://redis.io/docs/latest/operate/oss_and_stack/install/install-redis/install-redis-on-linux/
sudo apt-get install -y lsb-release curl gpg
curl -fsSL https://packages.redis.io/gpg | sudo gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg
sudo chmod 644 /usr/share/keyrings/redis-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/redis.list
sudo apt-get update
sudo apt-get install -y redis

# may require
sudo systemctl status redis-server
sudo systemctl start redis-server

# vim /etc/redis/redis.conf
# comment out the bind line
# set a pass via requirepass PW
#

sudo systemctl restart redis-server

# Then open default port 6379 in hetzner
# Then check install with
# ➜  ~ redis-cli -h 178.156.147.226 -p 6379 -a 'PW'
#


# GITEA setup
# https://docs.gitea.com/installation/install-from-package

snap install gitea

# Open port 3000 to YOUR IP ONLY
#
# config gitea
# add ssh key to user acct
vim /var/snap/gitea/common/conf/app.ini
# START_SSH_SERVER = true
# SSH_PORT = 2222
#
