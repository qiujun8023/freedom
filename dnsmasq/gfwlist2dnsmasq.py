#!/usr/bin/env python3

import re
import datetime
import time
import json
import base64
import urllib.request

CONFIG_PATH = './config.json'

COMMENT_PATTERN = '^\!|\[|^@@|^\d+\.\d+\.\d+\.\d+'
DOMAIN_PATTERN = '([\w\-\_]+\.[\w\.\-\_]+)[\/\*]*'

def now_time():
    return datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")

def generate_default_rules(config):
    with open(config['default_output_file'], 'w') as fs:
        for dns in config['default_dns']:
            fs.write('server=%s\n'%(dns))

def fetch_gfw_list(url, timeout = 30):
    content = urllib.request.urlopen(url, timeout = timeout).read()
    return str(base64.b64decode(content), encoding = "utf8")

def resolve_gfw_list(content):
    gfw_list = []
    for line in content.split('\n'):
        if re.findall(COMMENT_PATTERN, line):
            continue
        domain = re.findall(DOMAIN_PATTERN, line)
        if not domain:
            continue
        gfw_list.append(domain[0])
    return gfw_list

def generate_gfw_rules(config):
    # fetch gfw list
    gfw_list = config['gfw_list_ex']
    for url in config['gfw_list']:
        gfw_list.extend(resolve_gfw_list(fetch_gfw_list(url)))

    # write to file
    with open(config['gfw_output_file'], 'w') as fs:
        fs.write('# gfwlist for dnsmasq\n')
        fs.write('# updated on ' + now_time() + '\n\n')
        for domain in gfw_list:
            for dns in config['gfw_dns']:
                fs.write('server=/.%s/%s\n'%(domain, dns))

def main():
    with open(CONFIG_PATH) as f:
        config = json.loads(f.read())
    print('generate default rules')
    generate_default_rules(config)
    print('generate gfw rules')
    generate_gfw_rules(config)

if __name__ == '__main__':
    main()
