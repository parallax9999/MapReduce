"""
Ridge regression sufficient-statistics demo.

Input CSV (no header):
    x1,x2,...,xd,y

The framework passes each line (minus newline) as `value`.
"""

def map(key, value):
    """
    Map: for one training example, emit contributions to XtX and Xty.

    key: ignored (could be line number)
    value: "x1,x2,...,xd,y"
    """
    parts = value.strip().split(",")
    *features_str, y_str = parts

    # Features + bias
    x = [1.0] + [float(v) for v in features_str]  # x0 = 1
    y = float(y_str)
    d = len(x)

    # Contributions to X^T X
    for i in range(d):
        for j in range(d):
            k = f"XtX,{i},{j}"       # key encodes matrix position
            v = x[i] * x[j]
            yield (k, v)

    # Contributions to X^T y
    for i in range(d):
        k = f"Xty,{i}"
        v = x[i] * y
        yield (k, v)

def reduce(key, values):
    """
    Reduce: sum contributions for each key (matrix entry or vector entry).

    key: e.g. "XtX,0,1" or "Xty,2"
    values: list of floats
    """
    total = sum(values)
    
    yield (key, total)
