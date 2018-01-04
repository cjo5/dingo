	.section	__TEXT,__text,regular,pure_instructions
	.macosx_version_min 10, 11
	.globl	_printVec
	.align	4, 0x90
_printVec:                              ## @printVec
	.cfi_startproc
## BB#0:
	pushq	%rbp
Ltmp0:
	.cfi_def_cfa_offset 16
Ltmp1:
	.cfi_offset %rbp, -16
	movq	%rsp, %rbp
Ltmp2:
	.cfi_def_cfa_register %rbp
	subq	$48, %rsp
	movq	%rdi, -32(%rbp)
	movl	%esi, -24(%rbp)
	movq	-32(%rbp), %rdi
	movq	%rdi, -16(%rbp)
	movl	-24(%rbp), %esi
	movl	%esi, -8(%rbp)
	movl	-12(%rbp), %esi
	addl	-8(%rbp), %esi
	addl	-16(%rbp), %esi
	movl	%esi, %edi
	callq	_puti
	movl	%eax, -36(%rbp)         ## 4-byte Spill
	addq	$48, %rsp
	popq	%rbp
	retq
	.cfi_endproc

	.globl	_stuff
	.align	4, 0x90
_stuff:                                 ## @stuff
	.cfi_startproc
## BB#0:
	pushq	%rbp
Ltmp3:
	.cfi_def_cfa_offset 16
Ltmp4:
	.cfi_offset %rbp, -16
	movq	%rsp, %rbp
Ltmp5:
	.cfi_def_cfa_register %rbp
	subq	$120416, %rsp           ## imm = 0x1D660
	leaq	-80416(%rbp), %rdi
	movq	___stack_chk_guard@GOTPCREL(%rip), %rax
	movq	(%rax), %rax
	movq	%rax, -8(%rbp)
	callq	_printarr
	leaq	-120416(%rbp), %rdi
	callq	_printarr
	leaq	-416(%rbp), %rdi
	callq	_printarr
	movq	___stack_chk_guard@GOTPCREL(%rip), %rax
	movq	(%rax), %rax
	cmpq	-8(%rbp), %rax
	jne	LBB1_2
## BB#1:
	addq	$120416, %rsp           ## imm = 0x1D660
	popq	%rbp
	retq
LBB1_2:
	callq	___stack_chk_fail
	.cfi_endproc

	.globl	_main
	.align	4, 0x90
_main:                                  ## @main
	.cfi_startproc
## BB#0:
	pushq	%rbp
Ltmp6:
	.cfi_def_cfa_offset 16
Ltmp7:
	.cfi_offset %rbp, -16
	movq	%rsp, %rbp
Ltmp8:
	.cfi_def_cfa_register %rbp
	xorl	%eax, %eax
	movl	$0, -4(%rbp)
	popq	%rbp
	retq
	.cfi_endproc


.subsections_via_symbols
