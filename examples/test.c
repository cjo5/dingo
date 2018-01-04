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
        puti(v.v.x + v.v.y + v.z);
}


struct List {
        struct Vec2* v;
};

struct foo {
        const int x;
        int y;
};

void printarr(int* ptr);

//struct Vec3 globalVec3 = {.v.x = 5, .v.y = 13, .z = 9 };

void stuff() {
        int a[100];
        {
                int b[20000];
                printarr(b);
        }
        {
                int b[10000];
                printarr(b);
        }
        printarr(a);
}

int main() {

        
        return 0;
}