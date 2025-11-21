"""
Simple MapReduce word count implementation for CSV key-value pairs.
Takes CSV with format: word,count
Outputs aggregated counts per word.
"""

def map(key, value):
    yield (key, value)

def reduce(key, values):
    total_count = sum(values)
    yield (key, total_count)