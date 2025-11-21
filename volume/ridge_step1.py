def map(key, value):
    """
    Map: for one training example, emit contributions to XtX and Xty.

    key: ignored (could be line number)
    value: "x1,x2,...,xd,y"
    """
    parts = value.strip().split(",")
    *features_str, y_str = parts

    # Features + bias
    x = [1.0] + [float(v) for v in features_str]
    y = float(y_str)
    d = len(x)

    # Contributions to X^T X
    for i in range(d):
        for j in range(d):
            k = f"XtX_{i}_{j}"
            v = x[i] * x[j]
            yield (k, v)

    # Contributions to X^T y
    for i in range(d):
        k = f"Xty_{i}"
        v = x[i] * y
        yield (k, v)

def reduce(key, values):
    """
    Reduce: sum contributions for each key (matrix or vector entry).

    key: e.g. "XtX_1_0" or "Xty_0"
    values: list of strings like ["3.0", "5.0"] or floats
    """
    total = 0.0
    for v in values:
        total += float(v)
    yield (key, total)
