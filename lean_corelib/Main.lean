import Std
import Corelib
import Init.Data.Nat.Basic

open Nat
#print Nat
theorem solv (n:Nat): n+0= n := Nat.add_zero n

theorem succ_pred (n : Nat) : n ≠ 0 → succ (pred n) = n := by
  intro (hn : n ≠ 0)
  cases n with
  | zero => exact absurd rfl (hn : 0 ≠ 0)
  | succ n => rw [Nat.pred_succ]

example : m + 0 = m := by exact?
example : 0 + m = m := by exact?
--#print Real

--theorem mathd_algebra_455 (x : Real) (hO : 2 * (2 * (2 * (2 * x))) = 48) : x = 3 := sorry

def hello := "hji"
def main : IO Unit :=
  IO.println s!"Hello, {hello}!"
