#!/usr/bin/env python3
"""
Simple MapReduce word count implementation for CSV key-value pairs.
Takes CSV with format: word,count
Outputs aggregated counts per word.
"""

def map(key, value):
    """
    Map function for word count aggregation.
    
    Args:
        key: Word (e.g., "apple")
        value: Count (e.g., 3)
    
    Yields:
        (word, count) tuples - just pass through since we're aggregating
    """
    # Simply emit the key-value pair for the reduce phase to aggregate
    yield (key, value)

def reduce(key, values):
    """
    Reduce function for word count aggregation.
    
    Args:
        key: Word (e.g., "apple")
        values: List of counts for this word [3, 2, 1, 4]
    
    Yields:
        (word, total_count) tuple
    """
    total_count = sum(values)
    yield (key, total_count)

# Example usage for testing
if __name__ == "__main__":
    # Test data - key-value pairs from our CSV
    test_data = [
        ("apple", 3),
        ("banana", 5), 
        ("apple", 2),
        ("orange", 4),
        ("apple", 1)
    ]
    
    # Test map function
    print("=== Map Phase ===")
    map_results = []
    for word, count in test_data:
        for result in map(word, count):
            map_results.append(result)
            print(f"map('{word}', {count}) -> {result}")
    
    # Group by key for reduce phase
    from collections import defaultdict
    grouped = defaultdict(list)
    for word, count in map_results:
        grouped[word].append(count)
    
    # Test reduce function  
    print("\n=== Reduce Phase ===")
    for word, counts in grouped.items():
        for result in reduce(word, counts):
            print(f"reduce('{word}', {counts}) -> {result}")