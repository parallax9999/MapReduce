"""
MapReduce job #2: solve ridge regression from XtX/Xty entries.

Input CSV (from previous job):
    key,value
where key is "XtX_i_j" or "Xty_i" and value is a float.

Output CSV:
    w_0,<bias>
    w_1,<weight for x1>
    w_2,<weight for x2>
    ...
"""


def map(key, value):
    """
    Map for solve job.

    Input:
        key: e.g. "XtX_0_0" or "Xty_2"
        value: e.g. "5.0"

    We just funnel everything to a single reducer key "MODEL".
    """
    line = f"{key},{value}"
    # All entries go to the same logical key so one reducer sees everything
    yield ("MODEL", line)


def reduce(key, values):
    """
    Reduce for solve job.

    key: "MODEL"
    values: list of strings like "XtX_0_0,5.0" or "Xty_2,-20.0"
    """
    def solve_linear_system(A, b):
        """
        Solve A x = b for x using Gaussian elimination with partial pivoting.
        A: list of lists (n x n)
        b: list (n)
        Returns: list (n)
        """
        n = len(A)

        # Make deep copies so we don't mutate inputs
        M = [row[:] for row in A]
        rhs = b[:]

        # Forward elimination
        for k in range(n):
            # Pivot: find row with max |M[i][k]|
            pivot = k
            for i in range(k + 1, n):
                if abs(M[i][k]) > abs(M[pivot][k]):
                    pivot = i

            if abs(M[pivot][k]) < 1e-12:
                raise ValueError("Singular or nearly singular matrix")

            # Swap rows if needed
            if pivot != k:
                M[k], M[pivot] = M[pivot], M[k]
                rhs[k], rhs[pivot] = rhs[pivot], rhs[k]

            # Eliminate below
            for i in range(k + 1, n):
                factor = M[i][k] / M[k][k]
                # Update row i
                for j in range(k, n):
                    M[i][j] -= factor * M[k][j]
                rhs[i] -= factor * rhs[k]

        # Back substitution
        x = [0.0] * n
        for i in range(n - 1, -1, -1):
            s = 0.0
            for j in range(i + 1, n):
                s += M[i][j] * x[j]
            x[i] = (rhs[i] - s) / M[i][i]

        return x

    # Store entries in dictionaries first
    xtx = {}   # (i, j) -> float
    xty = {}   # i -> float
    max_index = -1

    for entry in values:
        # entry looks like "XtX_0_0,5.0"
        k_str, val_str = entry.split(",")
        val = float(val_str)

        parts = k_str.split("_")
        kind = parts[0]  # "XtX" or "Xty"

        if kind == "XtX":
            i = int(parts[1])
            j = int(parts[2])
            xtx[(i, j)] = val
            max_index = max(max_index, i, j)
        elif kind == "Xty":
            i = int(parts[1])
            xty[i] = val
            max_index = max(max_index, i)

    if max_index < 0:
        return  # nothing to solve

    d = max_index + 1

    # Build full dense XtX matrix and Xty vector
    XtX = [[0.0] * d for _ in range(d)]
    for (i, j), v in xtx.items():
        XtX[i][j] = v

    Xty = [0.0] * d
    for i, v in xty.items():
        Xty[i] = v

    # Ridge parameter (try 0.0 for exact solution; >0 to regularize)
    lam = 0.0

    # Don't regularize bias term (index 0)
    for i in range(1, d):
        XtX[i][i] += lam

    # Solve (XtX + lam R) w = Xty
    w = solve_linear_system(XtX, Xty)

    # Output weights as separate key/value pairs
    for i, wi in enumerate(w):
        yield (f"w_{i}", wi)

