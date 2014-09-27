#!/bin/bash -e -x
export SSH_KEYFILE="${SSH_KEYFILE:-$HOME/.ssh/isucon20140823.pem}"
export SSH_SERVER=isucon@54.64.206.61
export RSYNC_RSH="ssh -i $SSH_KEYFILE"


curl --data "deploy by $USER" 'https://teamfreesozai.slack.com/services/hooks/slackbot?token=oxjd47qGfo59VhemVz43FQZF&channel=%23general'
rsync -avz ./ $SSH_SERVER:/home/isucon/webapp/go
ssh -t -i $SSH_KEYFILE $SSH_SERVER bash -c 'env; cd /home/isucon/webapp/go && /home/isucon/env.sh /home/isucon/webapp/go/build.sh'

