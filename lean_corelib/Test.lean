-- This file is auto-generated and read-only. It cannot be modified manually.
import Corelib

open Function Nat

variable {a b c d m n k : ℕ} {p : ℕ → Prop}

example : ∀ n, n ≠ 0 ∧ n ≠ 1 → Nat.succ (Nat.pred n) = n := by exact?
