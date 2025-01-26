import Std
import Corelib

example : m + 0 = m := by exact?
example : 0 + m = m := by exact?
example : ∀ n, n ≠ 0 ∧ n ≠ 1 → Nat.succ (Nat.pred n) = n := by exact?
example : ∀ n, n ≠ 0 ∧ n ≠ 2 → Nat.succ (Nat.pred n) = n := by exact?
