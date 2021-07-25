#!/bin/bash

assert() {
  expected="$1"
  input="$2"

  ./main "$input" > tmp.s
  gcc -o tmp tmp.s
  ./tmp
  actual="$?"

  if [ "$actual" = "$expected" ]; then
    echo "$input => $actual"
  else
    echo "$input => $actual expected, but got $actual"
    exit 1
  fi
}

assert 0 0
assert 42 42
assert 21 "5+20-4"
assert 41 "12 + 34 - 5"
assert 47 '5 + 6*7'
assert 15 '5*(9-6)'
assert 4 '(3+5) / 2'
assert 5 '10 + -5'
assert 4 '-10 + -7 * -2'
assert 1 '1 + 1 == 2'
assert 0 '1 - 5 * 2 == 9'

echo OK