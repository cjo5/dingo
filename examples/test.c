int puts(const char* s);

int puti(int i);

int putsh(short i);
int putf(float i);

int putptr(int*);

int getf();

int geta();

int getb();

short getc();

static char* x = "hello";

struct Vec2 {
        int x;
        int y;
};

struct Vec3 {
        int z;
        struct Vec2 v;
};

void printVec(struct Vec3 v) {
}


struct List {
        struct Vec2* v;
};

struct foo {
        const int x;
        int y;
};

void _ZShit() {

}

static int main() {
        _ZShit();

        
        return 0;
}