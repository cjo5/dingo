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

namespace shit {
    void face() {

    }
}

void _Zshitface() {

}

int main() {
    _Zshitface();
    shit::face();
    return 0;
}