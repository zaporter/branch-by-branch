import math
import sys

args = sys.argv[1:]
nums = [float(arg) for arg in args]

print(nums)

mean = sum(nums) / len(nums)
variance = sum((x - mean) ** 2 for x in nums) / len(nums)
stddev = math.sqrt(variance)
print(f"mean: {mean}, variance: {variance}, stddev: {stddev}")

advantages = [(x - mean) / stddev for x in nums]

print(advantages)