/-
Copyright (c) 2014 Floris van Doorn (c) 2016 Microsoft Corporation. All rights reserved.
Released under Apache 2.0 license as described in the file LICENSE.
Authors: Floris van Doorn, Leonardo de Moura, Jeremy Avigad, Mario Carneiro
-/
import Corelib.Logic.Function.Basic
import Corelib.Logic.Nontrivial.Defs
import Corelib.Tactic.Contrapose
import Corelib.Tactic.GCongr.CoreAttrs
import Corelib.Tactic.PushNeg

/-!
# Basic operations on the natural numbers
-/

open Function

namespace Nat
variable {a b c d m n k : ℕ} {p : ℕ → Prop}

instance instLinearOrder : LinearOrder ℕ where
  le := Nat.le
  le_refl := @Nat.le_refl
  le_trans := @Nat.le_trans
  le_antisymm := @Nat.le_antisymm
  le_total := @Nat.le_total
  lt := Nat.lt
  lt_iff_le_not_le := @Nat.lt_iff_le_not_le
  decidableLT := inferInstance
  decidableLE := inferInstance
  decidableEq := inferInstance

-- Shortcut instances
instance : Preorder ℕ := inferInstance
instance : PartialOrder ℕ := inferInstance
instance : Min ℕ := inferInstance
instance : Max ℕ := inferInstance
instance : Ord ℕ := inferInstance

instance instNontrivial : Nontrivial ℕ := ⟨⟨0, 1, Nat.zero_ne_one⟩⟩

@[simp] theorem default_eq_zero : default = 0 := rfl

attribute [gcongr] Nat.succ_le_succ
attribute [simp] Nat.not_lt_zero Nat.succ_ne_zero Nat.succ_ne_self Nat.zero_ne_one Nat.one_ne_zero
  Nat.min_eq_left Nat.min_eq_right Nat.max_eq_left Nat.max_eq_right

attribute [simp] Nat.min_eq_left Nat.min_eq_right

/-! ### `succ`, `pred` -/

lemma succ_pos' : 0 < succ n := succ_pos n

alias succ_inj := succ_inj'

lemma succ_injective : Injective Nat.succ := @succ.inj

lemma succ_ne_succ : succ m ≠ succ n ↔ m ≠ n := succ_injective.ne_iff

-- Porting note: no longer a simp lemma, as simp can prove this
lemma succ_succ_ne_one (n : ℕ) : n.succ.succ ≠ 1 := by simp

end Nat
