#!/usr/bin/env python3
"""
Generate a large CSV file with word,count data for MapReduce testing.
"""

import random
import sys

def generate_csv(num_records=1000000, output_file="large_input.csv"):
    """
    Generate a CSV file with random word,count pairs.
    
    Args:
        num_records: Number of records to generate
        output_file: Output filename
    """
    # Pool of words to use (will create realistic word frequency distribution)
    common_words = [
        "the", "be", "to", "of", "and", "a", "in", "that", "have", "it",
        "for", "not", "on", "with", "he", "as", "you", "do", "at", "this",
        "but", "his", "by", "from", "they", "we", "say", "her", "she", "or",
        "an", "will", "my", "one", "all", "would", "there", "their", "what",
        "so", "up", "out", "if", "about", "who", "get", "which", "go", "me"
    ]
    
    uncommon_words = [
        "algorithm", "database", "network", "system", "process", "thread",
        "memory", "cache", "buffer", "queue", "stack", "tree", "graph",
        "node", "edge", "path", "route", "packet", "byte", "bit",
        "server", "client", "request", "response", "protocol", "socket",
        "stream", "file", "disk", "storage", "index", "query", "table",
        "row", "column", "key", "value", "hash", "map", "reduce"
    ]
    
    rare_words = [
        f"rare_word_{i}" for i in range(1000)
    ]
    
    all_words = common_words * 100 + uncommon_words * 10 + rare_words
    
    print(f"Generating {num_records:,} records...")
    
    with open(output_file, 'w') as f:
        # Don't write header for MapReduce input
        # f.write("word,count\n")
        
        for i in range(num_records):
            # Pick a random word
            word = random.choice(all_words)
            
            # Generate a random count (weighted towards smaller numbers)
            if random.random() < 0.7:
                count = random.randint(1, 10)
            elif random.random() < 0.9:
                count = random.randint(10, 100)
            else:
                count = random.randint(100, 1000)
            
            f.write(f"{word},{count}\n")
            
            # Progress indicator every 100k records
            if (i + 1) % 100000 == 0:
                print(f"  Generated {i + 1:,} records...")
    
    # Calculate file size
    import os
    size = os.path.getsize(output_file)
    size_mb = size / (1024 * 1024)
    
    print(f"\nGenerated {output_file}:")
    print(f"  Records: {num_records:,}")
    print(f"  Size: {size_mb:.2f} MB ({size:,} bytes)")
    print(f"  Unique words: ~{len(set(all_words)):,}")
    
    # Show sample records
    print("\nSample records:")
    with open(output_file, 'r') as f:
        for i, line in enumerate(f):
            if i < 5:
                print(f"  {line.strip()}")
            else:
                break

if __name__ == "__main__":
    # Check if custom size provided
    if len(sys.argv) > 1:
        try:
            num_records = int(sys.argv[1])
        except ValueError:
            print(f"Usage: {sys.argv[0]} [num_records]")
            print(f"Example: {sys.argv[0]} 1000000")
            sys.exit(1)
    else:
        num_records = 1000000  # Default 1 million records
    
    # Generate in same directory as script
    output_file = "large_input.csv"
    
    generate_csv(num_records, output_file)