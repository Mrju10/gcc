/* PR rtl-optimization/49095 */
/* { dg-do compile } */
/* { dg-options "-Os -fno-shrink-wrap -masm=att" } */
/* { dg-additional-options "-mregparm=2" { target ia32 } } */

void foo (void *);

int *
f1 (int *x)
{
  if (!--*x)
    foo (x);
  return x;
}

int
g1 (int x)
{
  if (!--x)
    foo ((void *) 0);
  return x;
}

#define F(T, OP, OPN) \
T *			\
f##T##OPN (T *x, T y)	\
{			\
  *x OP y;		\
  if (!*x)		\
    foo (x);		\
  return x;		\
}			\
			\
T			\
g##T##OPN (T x, T y)	\
{			\
  x OP y;		\
  if (!x)		\
    foo ((void *) 0);	\
  return x;		\
}			\
			\
T *			\
h##T##OPN (T *x)	\
{			\
  *x OP 24;		\
  if (!*x)		\
    foo (x);		\
  return x;		\
}			\
			\
T			\
i##T##OPN (T x, T y)	\
{			\
  x OP 24;		\
  if (!x)		\
    foo ((void *) 0);	\
  return x;		\
}

#define G(T) \
F (T, +=, plus)		\
F (T, -=, minus)	\
F (T, &=, and)		\
F (T, |=, or)		\
F (T, ^=, xor)

G (char)
G (short)
G (int)
G (long)

/* { dg-final { scan-assembler-not "test\[lq\]" } } */
/* The {f,h}{char,short,int,long}xor functions aren't optimized into
   a RMW instruction, so need load, modify and store.  FIXME eventually.  */
/* { dg-final { scan-assembler-times "\\(%eax\\), %" 12 { target { ia32 } } } } */
/* { dg-final { scan-assembler-times "\\(%\[re\]di\\), %" 8 { target { ! ia32 } } } } */
