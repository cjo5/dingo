; ModuleID = 'examples/test.c'
source_filename = "examples/test.c"
target datalayout = "e-m:o-i64:64-f80:128-n8:16:32:64-S128"
target triple = "x86_64-apple-macosx10.11.0"

%struct.Vec3 = type { i32, %struct.Vec2 }
%struct.Vec2 = type { i32, i32 }

; Function Attrs: nounwind ssp uwtable
define void @printVec(i64, i32) #0 {
  %3 = alloca %struct.Vec3, align 4
  %4 = alloca { i64, i32 }, align 4
  %5 = getelementptr inbounds { i64, i32 }, { i64, i32 }* %4, i32 0, i32 0
  store i64 %0, i64* %5, align 4
  %6 = getelementptr inbounds { i64, i32 }, { i64, i32 }* %4, i32 0, i32 1
  store i32 %1, i32* %6, align 4
  %7 = bitcast %struct.Vec3* %3 to i8*
  %8 = bitcast { i64, i32 }* %4 to i8*
  call void @llvm.memcpy.p0i8.p0i8.i64(i8* %7, i8* %8, i64 12, i32 4, i1 false)
  %9 = getelementptr inbounds %struct.Vec3, %struct.Vec3* %3, i32 0, i32 1
  %10 = getelementptr inbounds %struct.Vec2, %struct.Vec2* %9, i32 0, i32 0
  %11 = load i32, i32* %10, align 4
  %12 = getelementptr inbounds %struct.Vec3, %struct.Vec3* %3, i32 0, i32 1
  %13 = getelementptr inbounds %struct.Vec2, %struct.Vec2* %12, i32 0, i32 1
  %14 = load i32, i32* %13, align 4
  %15 = add nsw i32 %11, %14
  %16 = getelementptr inbounds %struct.Vec3, %struct.Vec3* %3, i32 0, i32 0
  %17 = load i32, i32* %16, align 4
  %18 = add nsw i32 %15, %17
  %19 = call i32 @puti(i32 %18)
  ret void
}

; Function Attrs: argmemonly nounwind
declare void @llvm.memcpy.p0i8.p0i8.i64(i8* nocapture, i8* nocapture readonly, i64, i32, i1) #1

declare i32 @puti(i32) #2

; Function Attrs: nounwind ssp uwtable
define void @stuff() #0 {
  %1 = alloca [100 x i32], align 16
  %2 = alloca [20000 x i32], align 16
  %3 = alloca [10000 x i32], align 16
  %4 = getelementptr inbounds [20000 x i32], [20000 x i32]* %2, i32 0, i32 0
  call void @printarr(i32* %4)
  %5 = getelementptr inbounds [10000 x i32], [10000 x i32]* %3, i32 0, i32 0
  call void @printarr(i32* %5)
  %6 = getelementptr inbounds [100 x i32], [100 x i32]* %1, i32 0, i32 0
  call void @printarr(i32* %6)
  ret void
}

declare void @printarr(i32*) #2

; Function Attrs: nounwind ssp uwtable
define void @_ZShit() #0 {
  ret void
}

; Function Attrs: nounwind ssp uwtable
define i32 @main() #0 {
  %1 = alloca i32, align 4
  store i32 0, i32* %1, align 4
  call void @_ZShit()
  ret i32 0
}

attributes #0 = { nounwind ssp uwtable "disable-tail-calls"="false" "less-precise-fpmad"="false" "no-frame-pointer-elim"="true" "no-frame-pointer-elim-non-leaf" "no-infs-fp-math"="false" "no-nans-fp-math"="false" "stack-protector-buffer-size"="8" "target-cpu"="core2" "target-features"="+cx16,+fxsr,+mmx,+sse,+sse2,+sse3,+ssse3" "unsafe-fp-math"="false" "use-soft-float"="false" }
attributes #1 = { argmemonly nounwind }
attributes #2 = { "disable-tail-calls"="false" "less-precise-fpmad"="false" "no-frame-pointer-elim"="true" "no-frame-pointer-elim-non-leaf" "no-infs-fp-math"="false" "no-nans-fp-math"="false" "stack-protector-buffer-size"="8" "target-cpu"="core2" "target-features"="+cx16,+fxsr,+mmx,+sse,+sse2,+sse3,+ssse3" "unsafe-fp-math"="false" "use-soft-float"="false" }

!llvm.module.flags = !{!0}
!llvm.ident = !{!1}

!0 = !{i32 1, !"PIC Level", i32 2}
!1 = !{!"Apple LLVM version 8.0.0 (clang-800.0.42.1)"}
