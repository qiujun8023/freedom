#!/usr/bin/env python
# -*- coding: utf-8 -*-

import urllib2
import re
import os
import json
import datetime
import shutil
import time

config_path = './config.json'

def dnsmasq(op):
    return os.popen('/etc/init.d/dnsmasq %s'%(op)).read()

def now_time():
    return datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")

def generate_default_rules(config):
    # write to temp
    with open(config['default_file']['temp'], 'w') as fs:
        for dns in config['default_dns']:
            fs.write('server=%s\n'%(dns))
    # move to target
    shutil.move(config['default_file']['temp'], config['default_file']['target'])

def fetch_gfw_list(url, timeout = 30):
    # match comments/title/whitelist/ip address
    comment_pattern = '^\!|\[|^@@|^\d+\.\d+\.\d+\.\d+'
    domain_pattern = '([\w\-\_]+\.[\w\.\-\_]+)[\/\*]*'
    # fetching list
    gfw_text = urllib2.urlopen(url, timeout = timeout).read().decode('base64')
    # pattern
    gfw_list = []
    for line in gfw_text.split('\n'):
        if re.findall(comment_pattern, line):
            continue
        domain = re.findall(domain_pattern, line)
        if not domain:
            continue
        gfw_list.append(domain[0])
    return gfw_list

def generate_gfw_rules(config):
    # fetch gfw list
    gfw_list = config['gfw_list_ex']
    for url in config['gfw_list']:
        gfw_list.extend(fetch_gfw_list(url))
    # write to temp
    with open(config['gfw_file']['temp'], 'w') as fs:
        fs.write('# gfwlist for dnsmasq\n')
        fs.write('# updated on ' + now_time() + '\n\n')
        for domain in gfw_list:
            for dns in config['gfw_dns']:
                fs.write('server=/.%s/%s\n'%(domain, dns))
    # move to target
    shutil.move(config['gfw_file']['temp'], config['gfw_file']['target'])

def fetch_ad_list(url, timeout = 30):
    ad_text = urllib2.urlopen(url, timeout = timeout).read()
    # pattern
    ad_list = []
    for line in ad_text.split('\n'):
        item = line.split()
        if not item or item[0] != '127.0.0.1':
            continue
        ad_list.append(item[1])
    return ad_list

def generate_ad_rules(config):
    # fetch gfw list
    ad_list = config['ad_list_ex']
    for url in config['ad_list']:
        ad_list.extend(fetch_ad_list(url))
    # write to temp
    with open(config['ad_file']['temp'], 'w') as fs:
        fs.write('# adlist for dnsmasq\n')
        fs.write('# updated on ' + now_time() + '\n\n')
        for domain in ad_list:
            fs.write('address=/%s/127.0.0.1\n'%(domain))
    # move to target
    shutil.move(config['ad_file']['temp'], config['ad_file']['target'])

def main():
    # load config
    with open(config_path) as f:
        config = json.loads(f.read())
    # generate default rules
    print 'generate default rules'
    generate_default_rules(config)
    while True:
        # generate gfw rules
        print 'generate gfw rules'
        generate_gfw_rules(config)
        # generate ad rules
        print 'generate ad rules'
        generate_ad_rules(config)
        # restart dnsmasq
        print 'restart dnsmasq'
        print dnsmasq('restart')
        # sleep
        print 'sleeping'
        time.sleep(config['update_interval'])

if __name__ == '__main__':
    main()
