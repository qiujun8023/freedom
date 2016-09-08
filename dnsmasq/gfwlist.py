import urllib2
import re
import os
import datetime
import shutil

# your clean dns
dns_host = '192.168.0.1'
dns_port = '5353'

# the url of gfwlist
gfwlist_url = 'https://raw.githubusercontent.com/gfwlist/gfwlist/master/gfwlist.txt'

# match comments/title/whitelist/ip address
comment_pattern = '^\!|\[|^@@|^\d+\.\d+\.\d+\.\d+'
domain_pattern = '([\w\-\_]+\.[\w\.\-\_]+)[\/\*]*'

out_file    = '/tmp/gfwlist.conf'
rules_file  = '/etc/dnsmasq.d/gfwlist.conf'
google_file = './google.list'

print('fetching list...')
gfwlist = urllib2.urlopen(gfwlist_url, timeout = 30).read().decode('base64').split('\n')

# add google domains
with open(google_file) as fs:
    for line in fs:
        gfwlist.append(line)

print('page content fetched, analysis...')
with open(out_file, 'w') as fs:
    fs.write('# gfwlist for dnsmasq\n')
    fs.write('# updated on ' + datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S") + '\n')
    fs.write('#\n')

    domainlist = []
    for line in gfwlist:
        if re.findall(comment_pattern, line):
            # print('this is a comment line: ' + line)
            # fs.write('#' + line)
            continue

        domain = re.findall(domain_pattern, line)
        if not domain:
            # print('no valid domain in this line: ' + line)
            continue

        try:
            domainlist.index(domain[0])
            # print(domain[0] + ' exists.')
        except ValueError:
            domainlist.append(domain[0])
            fs.write('server=/.%s/%s#%s\n'%(domain[0], dns_host, dns_port))
            # fs.write('ipset=/.%s/gfwlist\n'%domain[0])

print('moving generated file to dnsmasg directory...')
shutil.move(out_file, rules_file)

# print('restart dnsmasq...')
# print(os.popen('/etc/init.d/dnsmasq restart').read())

print('done!')
