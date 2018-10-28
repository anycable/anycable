#coding: utf-8

import sys
import os
import re
import argparse

def process_file(input_file):
	log = {}
	log['clients'] = []
	log['95per'] = []
	log['min'] = []
	log['med'] = []
	log['max'] = []
		
	for line in input_file:
		point = parse_line(line)
		if point:
			log['clients'].append(point['clients'])
			log['95per'].append(point['95per'])
			log['min'].append(point['min'])
			log['med'].append(point['med'])
			log['max'].append(point['max'])
	return log
			
def parse_line(line):
  # clients:  1000    95per-rtt: 1328ms    min-rtt:   2ms    median-rtt: 457ms    max-rtt: 1577ms
	matches = re.search('clients:\s+(\d+)\s+95per\-rtt:\s+(\d+)ms\s+min\-rtt:\s+(\d+)ms\s+median\-rtt:\s+(\d+)ms\s+max\-rtt:\s+(\d+)ms', line)
	if matches:
		return {
			'clients': int(matches.group(1)),
			'95per': int(matches.group(2)),
			'min': int(matches.group(3)),
			'med': int(matches.group(4)),
			'max': int(matches.group(5))
		}
	return False

def generate_plot(log, output):
	import matplotlib.patches as mpatches
	import matplotlib.pyplot as plt

	with plt.rc_context({'backend': 'Agg'}):

		fig = plt.figure()
		ax = fig.add_subplot(1, 1, 1)

		ax.plot(log['clients'], log['95per'], '-', lw=1, color='r', label='95 percentile')
		ax.plot(log['clients'], log['med'], '-', lw=1, color='green', dashes=[10, 5], label='Median')
		ax.plot(log['clients'], log['max'], '-', lw=1, color='grey', label='Max')

		ax.set_ylabel('RTT ms', color='r')
		ax.set_xlabel('clients num')
		ax.set_ylim(0., max(log['max']) * 1.1)

		handles, labels = ax.get_legend_handles_labels()
		ax.legend(handles, labels, bbox_to_anchor=(0.4, 1))

		ax.grid()

		fig.savefig(output)
	
if __name__ == "__main__":
	parser = argparse.ArgumentParser(description='Generate RTT chart')
	parser.add_argument('-i', dest='inputfile', type=argparse.FileType('r'), help='input file containing benchmark results', required=True)
	parser.add_argument('-o', dest='outputfile', type=argparse.FileType('w'), help='output file to write resulted chart PNG', required=True)

	args = parser.parse_args()
		
	data = process_file(args.inputfile)

	generate_plot(data, args.outputfile)

	print('Done')	
