import Corelib

-- s/Foo
example : ∀ n, n ≠ 0 ∧ n ≠ 1 → Nat.succ (Nat.pred n) = n := by exact?
-- e
