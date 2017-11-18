int puts(const char* s);

int foo(int a) {
        return a+1;
}

int foo(int a) {
        return a+2;
}


int puti(int i);

int putsh(short i);
int putf(float i);

int getf();

int geta();

int getb();

short getc();

static char* x = "hello";

int main() {
        int a = foo(2);
        unsigned short b = (unsigned short)a;
        return 0;
}