import unittest
import os
import rtt_plot

class TestRttPlot(unittest.TestCase):
	def test_parse_line(self):
		self.assertEqual(
      { 'clients': 1000, '95per': 1328, 'min': 2, 'med': 457, 'max': 1577 },
      rtt_plot.parse_line('   "clients:  1000    95per-rtt: 1328ms    min-rtt:   2ms    median-rtt: 457ms    max-rtt: 1577ms"')
    )
		self.assertFalse(
      rtt_plot.parse_line('2018/10/28 02:24:13 Missing received broadcasts: expected 23100000, got 23005351')
    )

if __name__ == '__main__':
    unittest.main()
