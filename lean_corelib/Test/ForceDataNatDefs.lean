import Corelib
open Function Nat
variable {a b c d m n k : ℕ} {p : ℕ → Prop}
--N: simple not 0 not 2
example : ∀ n, n ≠ 0 ∧ n ≠ 2 → Nat.succ (Nat.pred n) = n := by exact?
--N: one_lt_succ_succ
example : 1 < succ (succ n) := one_lt_succ_succ n
--N: not_succ_lt_self
example : ¬ succ n < n := not_succ_lt_self
--N: succ_le_iff
example : succ m ≤ n ↔ m < n := succ_le_iff
--N: le_succ_iff
example : m ≤ n.succ ↔ m ≤ n ∨ m = n.succ := le_succ_iff
--N: lt_iff_add_one_le
example : m < n ↔ m + 1 ≤ n := lt_iff_add_one_le
--N: le_of_pred_lt
example : pred m < n → m ≤ n := le_of_pred_lt
--N: lt_iff_le_pred
example : 0 < n → (m < n ↔ m ≤ n - 1) := lt_iff_le_pred
--N: le_of_pred_lt
example : pred m < n → m ≤ n := le_of_pred_lt
--N: lt_iff_add_one_le
example : m < n ↔ m + 1 ≤ n := lt_iff_add_one_le
--N: one_add_le_iff
example : 1 + m ≤ n ↔ m < n := one_add_le_iff
--N: one_lt_iff_ne_zero_and_ne_one
example : 1 < n ↔ n ≠ 0 ∧ n ≠ 1 := one_lt_iff_ne_zero_and_ne_one
--N: le_one_iff_eq_zero_or_eq_one
example : n ≤ 1 ↔ n = 0 ∨ n = 1 := le_one_iff_eq_zero_or_eq_one
--N: one_le_of_lt
example : a < b → 1 ≤ b := one_le_of_lt
--N: min_left_comm
example : (a b c : ℕ) → min a (min b c) = min b (min a c) := Nat.min_left_comm
--N: max_left_comm
example : (a b c : ℕ) → max a (max b c) = max b (max a c) := Nat.max_left_comm
--N: min_right_comm
example : (a b c : ℕ) → min (min a b) c = min (min a c) b := Nat.min_right_comm
--N: max_right_comm
example : (a b c : ℕ) → max (max a b) c = max (max a c) b := Nat.max_right_comm
--N: min_eq_zero_iff
example : min m n = 0 ↔ m = 0 ∨ n = 0 := min_eq_zero_iff
--N: max_eq_zero_iff
example : max m n = 0 ↔ m = 0 ∧ n = 0 := max_eq_zero_iff
--N: pred_one_add
example : (n : ℕ ) → pred (1 + n) = n := pred_one_add
--N: pred_eq_self_iff
example : n.pred = n ↔ n = 0 := pred_eq_self_iff
--N: pred_eq_of_eq_succ
example : m = n.succ → m.pred = n := pred_eq_of_eq_succ
--N: pred_eq_succ_iff
example : n - 1 = m + 1 ↔ n = m + 2 := pred_eq_succ_iff
--N: forall_lt_succ
example : (∀ m < n + 1, p m) ↔ (∀ m < n, p m) ∧ p n := forall_lt_succ
--N: exists_lt_succ
example : (∃ m < n + 1, p m) ↔ (∃ m < n, p m) ∨ p n := exists_lt_succ
--N: two_lt_of_ne
example : ∀ {n}, n ≠ 0 → n ≠ 1 → n ≠ 2 → 2 < n := two_lt_of_ne
--N: add_succ_sub_one
example : (m n : ℕ) → m + succ n - 1 = m + n := add_succ_sub_one
--N: succ_add_sub_one
example : (n m : ℕ) → succ m + n - 1 = m + n := succ_add_sub_one
--N: pred_sub
example : (n m : ℕ) → pred n - m = pred (n - m) := pred_sub
--N: self_add_sub_one
example : (n : ℕ) → n + (n - 1) = 2 * n - 1 := self_add_sub_one
--N: sub_one_add_self
example : (n : ℕ) → (n - 1) + n = 2 * n - 1 := sub_one_add_self
--N: self_add_pred
example : (n : ℕ) → n + pred n = (2 * n).pred := self_add_pred
--N: pred_add_self
example : (n : ℕ) → pred n + n = (2 * n).pred := pred_add_self
--N: pred_le_iff
example : pred m ≤ n ↔ m ≤ succ n := pred_le_iff
--N: lt_of_lt_pred
example : m < n - 1 → m < n := lt_of_lt_pred
--N: le_add_pred_of_pos
example : (a : ℕ) → (hb : b ≠ 0) → a ≤ b + (a - 1) := le_add_pred_of_pos
--N: add_eq left and right
example : a + b = a ↔ b = 0 := Nat.add_eq_left
example : a + b = b ↔ a = 0 := Nat.add_eq_right
--N: two_le_iff
example : (n : ℕ) → 2 ≤ n ↔ n ≠ 0 ∧ n ≠ 1 := two_le_iff
--N: add_eq_maxmin_iff
example : m + n = max m n ↔ m = 0 ∨ n = 0 := add_eq_max_iff
example : m + n = min m n ↔ m = 0 ∧  n = 0 := add_eq_min_iff
--N: add_eq_zero
example : m + n = 0 ↔ m = 0 ∧ n = 0 := Nat.add_eq_zero
--N: two_mul_ne_two_mul_add_one
example : 2 * n ≠ 2 * m + 1 := two_mul_ne_two_mul_add_one
--N: mul_def
example : Nat.mul m n = m * n := Nat.mul_def
--N: zero_eq_mul
example : 0 = m * n ↔ m = 0 ∨ n = 0 := Nat.zero_eq_mul
--N: mul_eq_left
example : (ha : a ≠ 0) → a * b = a ↔ b = 1 := mul_eq_left
--N: mul_eq_right
example : (hb : b ≠ 0) → a * b = b ↔ a = 1 := mul_eq_right
--N: mul_right_eq_self_iff
example : (ha : 0 < a) → a * b = a ↔ b = 1 := mul_right_eq_self_iff
--N: mul_left_eq_self_iff
example : (hb : 0 < b) → a * b = b ↔ a = 1 := mul_left_eq_self_iff
--N: le_of_mul_le_mul_right
example : (h : a * c ≤ b*c) →(hc : 0 < c) → a ≤ b := Nat.le_of_mul_le_mul_right
--N: one_lt_mul_iff
example : 1 < m * n ↔ 0 < m ∧ 0 < n ∧ (1 < m ∨ 1 < n) := one_lt_mul_iff
--N: eq_one_of_mul_eq_one right and left
example : (H : m * n = 1) → m = 1 := eq_one_of_mul_eq_one_right
example : (H : m * n = 1) → n = 1 := eq_one_of_mul_eq_one_left
--N: lt_mul_iff_one_lt_left and right
example : (hb : 0 < b) → b < a * b ↔ 1 < a := Nat.lt_mul_iff_one_lt_left
example : (ha : 0 < a) → a < a * b ↔ 1 < b := Nat.lt_mul_iff_one_lt_right
--N: eq_zero_of_double_le
example : (h : 2 * n ≤ n) → n = 0 := Nat.eq_zero_of_double_le
--N: eq_zero_of_mul_le
example : (hb : 2 ≤ n) → (h : n * m ≤ m) → m = 0 := Nat.eq_zero_of_mul_le
--N: succ_mul_pos
example : (m : ℕ) → (hn : 0 < n) → 0 < succ m * n := Nat.succ_mul_pos
--N: mul_self_le_mul_self
example : (h : m ≤ n) → m * m ≤ n * n := Nat.mul_self_le_mul_self
--N: mul_lt_mul''
example : (hac : a < c) → (hbd : b < d) → a * b < c * d := Nat.mul_lt_mul''
--N: mul_self_lt_mul_self
example : (h : m < n) → m * m < n * n := Nat.mul_self_lt_mul_self
--N: mul_self_le_mul_self_iff
example : m * m ≤ n * n ↔ m ≤ n := Nat.mul_self_le_mul_self_iff
--N: mul_self_lt_mul_self_iff
example : m * m < n * n ↔ m < n := Nat.mul_self_lt_mul_self_iff
--N: le_mul_self
example : (n : ℕ) → n ≤ n * n := Nat.le_mul_self
--N: mul_self_inj
example : m * m = n * n ↔ m = n := Nat.mul_self_inj
--N: lt_mul_self_iff
example : ∀ {n : ℕ}, n < n * n ↔ 1 < n := lt_mul_self_iff
--N: add_sub_one_le_mul
example : (ha : a ≠ 0) → (hb : b ≠ 0) → a + b - 1 ≤ a * b := Nat.add_sub_one_le_mul
--N: add_le_mul
example :  {a : ℕ} → (ha : 2 ≤ a) → ∀ {b : ℕ} (_ : 2 ≤ b), a + b ≤ a * b := Nat.add_le_mul
--N: div_le_iff_le_mul_add_pred
example : (hb : 0 < b) → a / b ≤ c ↔ a ≤ b * c + (b - 1) := Nat.div_le_iff_le_mul_add_pred
--N: div_lt_iff_lt_mul
example : (hb : 0 < b) → a / b < c ↔ a < c * b := Nat.div_lt_iff_lt_mul
--N: one_le_div_iff
example : (hb : 0 < b) → 1 ≤ a / b ↔ b ≤ a := Nat.one_le_div_iff
--N: div_lt_one_iff
example : (hb : 0 < b) → a / b < 1 ↔ a < b := Nat.div_lt_one_iff
--N: div_le_div_right
example : (h : a ≤ b) → a / c ≤ b / c := Nat.div_le_div_right
--N: lt_of_div_lt_div
example : (h : a / c < b / c) → a < b := Nat.lt_of_div_lt_div
--N: div_eq_zero_iff
example : a / b = 0 ↔ b = 0 ∨ a < b := Nat.div_eq_zero_iff
--N: div_ne_zero_iff
example : a / b ≠ 0 ↔ b ≠ 0 ∧ b ≤ a := Nat.div_ne_zero_iff
--N: div_pos_iff
example : 0 < a / b ↔ 0 < b ∧ b ≤ a := Nat.div_pos_iff
--N: div_pos
example : (hba : b ≤ a) → (hb : 0 < b) → 0 < a / b := Nat.div_pos
--N: lt_mul_of_div_lt
example : (h : a / c < b) → (hc : 0 < c) → a < b * c := Nat.lt_mul_of_div_lt
--N: mul_div_le_mul_div_assoc
example : (a b c : ℕ) → a * (b / c) ≤ a * b / c := Nat.mul_div_le_mul_div_assoc
--N: div_left_inj
example : (hda : d ∣ a) → (hdb : d ∣ b) → a / d = b / d ↔ a = b := Nat.div_left_inj
--N: div_mul_div_comm
example : (hba : b ∣ a) → (hdc : d ∣ c) → (a / b) * (c / d) = (a * c) / (b * d) := Nat.div_mul_div_comm
--N: eq_mul_of_div_eq_left
example : (H1 : b ∣ a) → (H2 : a / b = c) → a = c * b := Nat.eq_mul_of_div_eq_left
--N: mul_div_cancel_left'
example : (Hd : a ∣ b) → a * (b / a) = b := Nat.mul_div_cancel_left'
--N: lt_div_mul_add
example : (hb : 0 < b) → a < a / b * b + b := Nat.lt_div_mul_add
