import LeanPlayground
import Std
import Init.Data.Nat.Basic

#print Nat
theorem solv (n:Nat): n+0= n := Nat.add_zero n

example : m + 0 = m := solv m

def main : IO Unit :=
  IO.println s!"Hello, {hello}!"
