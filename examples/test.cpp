int geta();

int a = geta();

extern "C" void printarr(int*);

class Foo {
    int a;
public:
    Foo() {
        a = 9;
    }


    ~Foo() {
        a = 5;
    }
    
};

extern "C" void printfoo(Foo*);

int main() {
        Foo f1;
        {
            Foo f2;
            printfoo(&f1);
        }
        printfoo(&f1);

}