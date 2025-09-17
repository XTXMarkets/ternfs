// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <stdio.h>
#include <stdlib.h>
#include <string>
#include <limits.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <string.h>
#include <sys/stat.h>
#include <time.h>
#include <stdint.h>
#include <sys/mman.h>

#define die(fmt, ...) do { fprintf(stderr, fmt "\n" __VA_OPT__(,) __VA_ARGS__); exit(1); } while(false)

const char* exe = NULL;

#define badUsage(...) do { \
        fprintf(stderr, "Bad usage, expecting %s writefile|readfile|readlink <command arguments>\n", exe); \
        __VA_OPT__(fprintf(stderr, __VA_ARGS__); fprintf(stderr, "\n");) \
        exit(2); \
    } while(0) \

static uint64_t nanosNow() {
    struct timespec tp;
    if (clock_gettime(CLOCK_REALTIME, &tp) < 0) {
        die("could not get time: %d (%s)", errno, strerror(errno));
    }
    return tp.tv_sec*1000000000ull + tp.tv_nsec;
}

// Just a super dumb file test, to have a controlled environment
// where every syscall is accounted for.
static void writeFile(int argc, const char** argv) {
    ssize_t fileSize = -1;
    ssize_t bufSize = -1; // if -1, all in one go
    const char* filename = NULL;
    bool closeAndReopen = false;
    bool stat = false;

    for (int i = 0; i < argc; i++) {
        if (std::string(argv[i]) == "-buf-size") {
            if (i+1 >= argc) { badUsage("No argument after -buf-size"); } i++;
            bufSize = strtoull(argv[i], NULL, 0);
            if (bufSize == ULLONG_MAX) {
                badUsage("Bad -buf-size: %d (%s)", errno, strerror(errno));
            }
        } else if (std::string(argv[i]) == "-size") {
            if (i+1 >= argc) { badUsage("No argument after -size"); } i++;
            fileSize = strtoull(argv[i], NULL, 0);
            if (fileSize == ULLONG_MAX) {
                badUsage("Bad -size: %d (%s)", errno, strerror(errno));
            }
        } else if (std::string(argv[i]) == "-close-open") {
            closeAndReopen = true;
        } else if (std::string(argv[i]) == "-stat") {
            stat = true;
        } else {
            if (filename != NULL) { badUsage("Filename already specified: %s", filename); }
            filename = argv[i];
        }
    }

    if (bufSize < 0) { bufSize = fileSize; }
    if (fileSize < 0 || filename == NULL) { badUsage("No -size specified"); }

    printf("writing %ld bytes with bufsize %ld to %s\n", fileSize, bufSize, filename);

    int fd = open(filename, O_CREAT | O_WRONLY | O_TRUNC, 0666);
    if (fd < 0) {
        die("could not open file %s: %d (%s)", filename, errno, strerror(errno));
    }

    if (closeAndReopen) {
        if (close(fd) < 0) {
            die("could not close file %s: %d (%s)", filename, errno, strerror(errno));
        }
        fd = open(filename, O_CREAT | O_WRONLY | O_TRUNC, 0666);
        if (fd < 0) {
            die("could not reopen file %s: %d (%s)", filename, errno, strerror(errno));
        }
    }

    uint8_t* buffer = (uint8_t*)malloc(bufSize);
    if (buffer == NULL) {
        die("could not allocate: %d (%s)", errno, strerror(errno));
    }

    uint64_t start = nanosNow();

    ssize_t toWrite = fileSize;
    while (toWrite > 0) {
        ssize_t res = write(fd, buffer, toWrite > bufSize ? bufSize : toWrite);
        if (res < 0) {
            die("couldn't write %s: %d (%s)", filename, errno, strerror(errno));
        }
        toWrite -= res;
    }

    // we should be able to stat a file open for writing
    if (stat) {
        struct stat st;
        if(fstat(fd, &st) != 0) {
            die("couldn't fstat %s: %d (%s)", filename, errno, strerror(errno));
        }
        char time_str[100];
        struct tm *tm_info = localtime(&st.st_mtime);
        strftime(time_str, sizeof(time_str), "%Y-%m-%d %H:%M:%S", tm_info);
        printf("stat size: %ld, mtime: %s\n", st.st_size, time_str);
    }

    printf("finished writing, will now close\n");

    if (close(fd) < 0) {
        die("couldn't close %s: %d (%s)", filename, errno, strerror(errno));
    }

    uint64_t elapsed = nanosNow() - start;
    printf("done (%fGB/s).\n", (double)fileSize/(double)elapsed);
}

// Same as writeFile, but for reading.
static void readFile(int argc, const char** argv) {
    ssize_t bufSize = -1; // if -1, all in one go
    ssize_t begin = -1; // if -1, start of file
    ssize_t end = -1; // if -1, end of file
    bool useMmap = false;
    bool mmapRandom = false;
    bool backwards = false;
    const char* filename = NULL;

    for (int i = 0; i < argc; i++) {
        if (std::string(argv[i]) == "-buf-size") {
            if (i+1 >= argc) { badUsage("No argument after -buf-size"); } i++;
            bufSize = strtoull(argv[i], NULL, 0);
            if (bufSize == ULLONG_MAX) {
                badUsage("Bad -buf-size: %d (%s)", errno, strerror(errno));
            }
        } else if (std::string(argv[i]) == "-begin") {
            if (i+1 >= argc) { badUsage("No argument after -begin"); } i++;
            begin = strtoull(argv[i], NULL, 0);
            if (begin == ULLONG_MAX) {
                badUsage("Bad -begin: %d (%s)", errno, strerror(errno));
            }
        } else if (std::string(argv[i]) == "-end") {
            if (i+1 >= argc) { badUsage("No argument after -end"); } i++;
            end = strtoull(argv[i], NULL, 0);
            if (end == ULLONG_MAX) {
                badUsage("Bad -end: %d (%s)", errno, strerror(errno));
            }
        } else if (std::string(argv[i]) == "-mmap") {
            useMmap = true;
        } else if (std::string(argv[i]) == "-random") {
            mmapRandom = true;
        } else if (std::string(argv[i]) == "-backwards") {
            backwards = true;
        } else {
            if (filename != NULL) { badUsage("Filename already specified: %s", filename); }
            filename = argv[i];
        }
    }

    if (backwards && !useMmap) {
        die("-backwards only works with -mmap for now");
    }

    size_t fileSize;
    {
        struct stat st;
        if(stat(filename, &st) != 0) {
            die("couldn't stat %s: %d (%s)", filename, errno, strerror(errno));
        }
        fileSize = st.st_size;
    }

    if (bufSize < 0) { bufSize = fileSize; }
    if (begin < 0) { begin = 0; }
    if (end < 0) { end = fileSize; }
    if (end > fileSize) {
        die("-end (%ld) exceeds file size (%lu)", end, fileSize);
    }

    printf("reading %ld bytes with bufsize %ld to %s\n", end-begin, bufSize, filename);

    int fd = open(filename, O_RDONLY);
    if (fd < 0) {
        die("could not open file %s: %d (%s)", filename, errno, strerror(errno));
    }

    uint64_t start;
    uint64_t elapsed;

    if (useMmap) {
        long pageSize = sysconf(_SC_PAGE_SIZE);
        if (pageSize < 0) {
            die("could not get page size: %d (%s)", errno, strerror(errno));
        }
        if (begin%pageSize != 0) {
            die("begin=%ld is not a multiple of page size %ld", begin, pageSize);
        }
        printf("mmapping region of size %ld\n", end-begin);
        uint8_t* data = (uint8_t*)mmap(nullptr, end-begin, PROT_READ, MAP_PRIVATE, fd, begin);
        if (data == MAP_FAILED) {
            die("could not mmap %s: %d (%s)", filename, errno, strerror(errno));
        }
        if (mmapRandom) {
            int ret = posix_madvise(data, end-begin, POSIX_MADV_RANDOM);
            if (ret != 0) {
                die("could not posix_madvise: %d (%s)", ret, strerror(ret));
            }
        }
        start = nanosNow();
        if (backwards) {
            for (ssize_t cursor = pageSize * ((end-1)/pageSize); cursor >= begin; cursor -= pageSize) {
                volatile uint8_t x = data[cursor-begin];
            }
        } else {
            for (ssize_t cursor = begin; cursor < end; cursor += pageSize) {
                volatile uint8_t x = data[cursor-begin];
            }
        }
        elapsed = nanosNow() - start;
        if (munmap(data, end-begin) < 0) {
            die("could not munmap: %d (%s)", errno, strerror(errno));
        }
    } else {
        uint8_t* buffer = (uint8_t*)malloc(bufSize);
        if (buffer == NULL) {
            die("could not allocate: %d (%s)", errno, strerror(errno));
        }

        start = nanosNow();
        
        if (lseek(fd, begin, SEEK_SET) < 0) {
            die("could not seek: %d (%s)", errno, strerror(errno));
        }

        size_t readSize = 0;
        for (;;) {
            ssize_t ret = read(fd, buffer, std::min<ssize_t>(bufSize, (end-begin)-readSize));
            if (ret < 0) {
                die("could not read file %s: %d (%s)", filename, errno, strerror(errno));
            }
            if (ret == 0) { break; }
            readSize += ret;
        }

        if (readSize != end-begin) {
            die("expected to read %lu, but read %lu instead", end-begin, readSize);
        }

        elapsed = nanosNow() - start;
    }

    printf("finished reading, will now close\n");

    if (close(fd) < 0) {
        die("couldn't close %s: %d (%s)", filename, errno, strerror(errno));
    }

    printf("done (%fGB/s).\n", (double)(end-begin)/(double)elapsed);
}

// Same as writeFile, but for reading.
static void readLink(int argc, const char** argv) {
    const char* filename = NULL;

    for (int i = 0; i < argc; i++) {
        if (filename != NULL) { badUsage("Filename already specified: %s", filename); }
        filename = argv[i];
    }

    char buf[1024];
    ssize_t ret = readlink(filename, buf, sizeof(buf));
    if (ret < 0) {
        die("could not readlink %s: %d (%s)", filename, errno, strerror(errno));
    }
    
    printf("link: \"");
    for (int i = 0; i < ret; i++) {
        if (isprint(buf[i])) {
            printf("%c", buf[i]);
        } else {
            printf("\\%02x", buf[i]);
        }
    }
    printf("\"\n");
}

static void printDelta(int64_t nanos) {
    if (nanos < 1'000ull) {
        printf("%ldns", nanos);
    } else if (nanos < 1'000'000ull) {
        printf("%.2lfus", ((double)nanos)/1e3);
    } else if (nanos < 1'000'000'000ull) {
        printf("%.2lfms", ((double)nanos)/1e6);
    } else {
        printf("%.2lfs", ((double)nanos)/1e9);
    }
}

// right now just used for timing
static void statFile(int argc, const char** argv) {
    const char* filename = NULL;
    uint64_t iterations = 1;

    for (int i = 0; i < argc; i++) {
        if (std::string(argv[i]) == "-iterations") {
            if (i+1 >= argc) { badUsage("No argument after -iterations"); } i++;
            iterations = strtoull(argv[i], NULL, 0);
            if (iterations == ULLONG_MAX) {
                badUsage("Bad -iterations: %d (%s)", errno, strerror(errno));
            }
        } else {
            if (filename != NULL) { badUsage("Filename already specified: %s", filename); }
            filename = argv[i];
        }
    }

    printf("will stat %lu times\n", iterations);

    int64_t avg = 0;

    for (uint64_t i = 0; i < iterations; i++) {
        struct timespec ts0;
        clock_gettime(CLOCK_REALTIME, &ts0);

        {
            struct stat st;
            if(stat(filename, &st) != 0) {
                die("couldn't stat %s: %d (%s)", filename, errno, strerror(errno));
            }
        }
        struct timespec ts1;
        clock_gettime(CLOCK_REALTIME, &ts1);

        int64_t t0 = ts0.tv_sec * 1'000'000'000ull + ts0.tv_nsec;
        int64_t t1 = ts1.tv_sec * 1'000'000'000ull + ts1.tv_nsec;
        int64_t d = t1 - t0;

        printf("%lu:\t", i);
        printDelta(d);
        printf("\n");

        avg += d/iterations;
    }

    printf("avg:\t");
    printDelta(avg);
    printf("\n");
}

int main(int argc, const char** argv) {
    exe = argv[0];

    if (argc < 2) { badUsage("No command"); }

    std::string cmd(argv[1]);

    if (cmd == "writefile") {
        writeFile(argc - 2, argv + 2);
    } else if (cmd == "readfile") {
        readFile(argc - 2, argv + 2);
    } else if (cmd == "readlink") {
        readLink(argc - 2, argv + 2); 
    } else if (cmd == "stat") {
        statFile(argc - 2, argv + 2); 
    } else {
        badUsage("Bad command %s", cmd.c_str());
    }

    return 0;
}