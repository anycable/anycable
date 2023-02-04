#coding: utf-8

import sys
import os
import re
import argparse

def process_file(input_file):
	points = []

	for line in input_file:
		points.append(float(line))

	return points


def generate_plot(log, output):
	import matplotlib.patches as mpatches
	import matplotlib.pyplot as plt

	with plt.rc_context({'backend': 'Agg'}):

		fig = plt.figure()
		ax = fig.add_subplot(1, 1, 1)

		ax.plot(log, lw=1, color='r', label='')
		ax.set_ylabel('RAM (MiB)', color='r')
		ax.grid()

		fig.savefig(output)

if __name__ == "__main__":
	parser = argparse.ArgumentParser(description='Generate RTT chart')
	parser.add_argument('-i', dest='inputfile', type=argparse.FileType('r'), help='input file containing benchmark results', required=True)
	parser.add_argument('-o', dest='outputfile', help='output file path to write resulted chart PNG', required=True)

	args = parser.parse_args()

	data = process_file(args.inputfile)

	generate_plot(data, args.outputfile)

	print('Done')	
