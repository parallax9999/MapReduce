#!/usr/bin/env python3
import random

def main():
    random.seed(42)  # for reproducibility

    output_file = "ridge_demo_data.csv"
    num_rows = 10000
    num_features = 6

    # True parameters for the data-generating process
    # y = bias + w1*x1 + ... + w6*x6 + noise
    bias = 4.0
    weights = [3.0, -2.0, 1.5, 0.5, -1.0, 2.5]  # length 6

    assert len(weights) == num_features

    with open(output_file, "w") as f:
        for i in range(1, num_rows + 1):
            # Generate feature vector x ~ Uniform(-5, 5)^6
            x = [random.uniform(-5.0, 5.0) for _ in range(num_features)]

            # Linear combination + Gaussian noise
            linear_part = sum(w * xi for w, xi in zip(weights, x)) + bias
            noise = random.gauss(0.0, 1.0)  # N(0, 1) noise
            y = linear_part + noise

            # Build row: id, x1..x6, y
            row_id = f"id{i}"
            # Format with a reasonable number of decimals
            feature_strs = [f"{xi:.5f}" for xi in x]
            y_str = f"{y:.5f}"

            line = ",".join([row_id] + feature_strs + [y_str])
            f.write(line + "\n")

    print(f"Wrote {num_rows} rows to {output_file}")
    print("True parameters:")
    print("  bias =", bias)
    print("  weights =", weights)

if __name__ == "__main__":
    main()

