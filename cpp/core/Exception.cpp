// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <signal.h>
#include <poll.h>
#include <sys/syscall.h>
#include <atomic>
#include <fcntl.h>
#include <cxxabi.h>

#include "Common.hpp"
#include "BinaryFormatter.hpp"
#include "Exception.hpp"

const char *translateErrno(int _errno) {
    switch (_errno) {
    case 1:   return "EPERM";
    case 2:   return "ENOENT";
    case 3:   return "ESRCH";
    case 4:   return "EINTR";
    case 5:   return "EIO";
    case 6:   return "ENXIO";
    case 7:   return "E2BIG";
    case 8:   return "ENOEXEC";
    case 9:   return "EBADF";
    case 10:  return "ECHILD";
    case 11:  return "EAGAIN";
    case 12:  return "ENOMEM";
    case 13:  return "EACCES";
    case 14:  return "EFAULT";
    case 15:  return "ENOTBLK";
    case 16:  return "EBUSY";
    case 17:  return "EEXIST";
    case 18:  return "EXDEV";
    case 19:  return "ENODEV";
    case 20:  return "ENOTDIR";
    case 21:  return "EISDIR";
    case 22:  return "EINVAL";
    case 23:  return "ENFILE";
    case 24:  return "EMFILE";
    case 25:  return "ENOTTY";
    case 26:  return "ETXTBSY";
    case 27:  return "EFBIG";
    case 28:  return "ENOSPC";
    case 29:  return "ESPIPE";
    case 30:  return "EROFS";
    case 31:  return "EMLINK";
    case 32:  return "EPIPE";
    case 33:  return "EDOM";
    case 34:  return "ERANGE";
    case 35:  return "EDEADLK";
    case 36:  return "ENAMETOOLONG";
    case 37:  return "ENOLCK";
    case 38:  return "ENOSYS";
    case 39:  return "ENOTEMPTY";
    case 40:  return "ELOOP";
    case 42:  return "ENOMSG";
    case 43:  return "EIDRM";
    case 44:  return "ECHRNG";
    case 45:  return "EL2NSYNC";
    case 46:  return "EL3HLT";
    case 47:  return "EL3RST";
    case 48:  return "ELNRNG";
    case 49:  return "EUNATCH";
    case 50:  return "ENOCSI";
    case 51:  return "EL2HLT";
    case 52:  return "EBADE";
    case 53:  return "EBADR";
    case 54:  return "EXFULL";
    case 55:  return "ENOANO";
    case 56:  return "EBADRQC";
    case 57:  return "EBADSLT";
    case 59:  return "EBFONT";
    case 60:  return "ENOSTR";
    case 61:  return "ENODATA";
    case 62:  return "ETIME";
    case 63:  return "ENOSR";
    case 64:  return "ENONET";
    case 65:  return "ENOPKG";
    case 66:  return "EREMOTE";
    case 67:  return "ENOLINK";
    case 68:  return "EADV";
    case 69:  return "ESRMNT";
    case 70:  return "ECOMM";
    case 71:  return "EPROTO";
    case 72:  return "EMULTIHOP";
    case 73:  return "EDOTDOT";
    case 74:  return "EBADMSG";
    case 75:  return "EOVERFLOW";
    case 76:  return "ENOTUNIQ";
    case 77:  return "EBADFD";
    case 78:  return "EREMCHG";
    case 79:  return "ELIBACC";
    case 80:  return "ELIBBAD";
    case 81:  return "ELIBSCN";
    case 82:  return "ELIBMAX";
    case 83:  return "ELIBEXEC";
    case 84:  return "EILSEQ";
    case 85:  return "ERESTART";
    case 86:  return "ESTRPIPE";
    case 87:  return "EUSERS";
    case 88:  return "ENOTSOCK";
    case 89:  return "EDESTADDRREQ";
    case 90:  return "EMSGSIZE";
    case 91:  return "EPROTOTYPE";
    case 92:  return "ENOPROTOOPT";
    case 93:  return "EPROTONOSUPPORT";
    case 94:  return "ESOCKTNOSUPPORT";
    case 95:  return "EOPNOTSUPP";
    case 96:  return "EPFNOSUPPORT";
    case 97:  return "EAFNOSUPPORT";
    case 98:  return "EADDRINUSE";
    case 99:  return "EADDRNOTAVAIL";
    case 100: return "ENETDOWN";
    case 101: return "ENETUNREACH";
    case 102: return "ENETRESET";
    case 103: return "ECONNABORTED";
    case 104: return "ECONNRESET";
    case 105: return "ENOBUFS";
    case 106: return "EISCONN";
    case 107: return "ENOTCONN";
    case 108: return "ESHUTDOWN";
    case 109: return "ETOOMANYREFS";
    case 110: return "ETIMEDOUT";
    case 111: return "ECONNREFUSED";
    case 112: return "EHOSTDOWN";
    case 113: return "EHOSTUNREACH";
    case 114: return "EALREADY";
    case 115: return "EINPROGRESS";
    case 116: return "ESTALE";
    case 117: return "EUCLEAN";
    case 118: return "ENOTNAM";
    case 119: return "ENAVAIL";
    case 120: return "EISNAM";
    case 121: return "EREMOTEIO";
    case 122: return "EDQUOT";
    case 123: return "ENOMEDIUM";
    case 124: return "EMEDIUMTYPE";
    case 125: return "ECANCELED";
    case 126: return "ENOKEY";
    case 127: return "EKEYEXPIRED";
    case 128: return "EKEYREVOKED";
    case 129: return "EKEYREJECTED";
    case 130: return "EOWNERDEAD";
    case 131: return "ENOTRECOVERABLE";
    case 132: return "ERFKILL";
    case 133: return "EHWPOISON";
    default:  return "????";
    }
}

std::string removeTemplates(const std::string & s) {
    std::string r = s;

    // Remove template arguments to class names
    // FIXME this also picks up less than and greater than signs,
    //       which needs fixing
    size_t begin = 0;
    int stack = 0;
    for (size_t i=0; i<r.size(); ++i) {
        if (r[i] == '<') {
            if (stack == 0) begin = i;
            ++stack;
        } else if (r[i] == '>') {
            if (stack == 0) continue;
            --stack;
            if (stack == 0) {
                r = r.substr(0, begin) + r.substr(i+1);
                i = begin;
            }
        }
    }

    // Remove any trailing [with args ...] sections
    size_t with_args = r.find(" [with args");
    if (with_args != std::string::npos && r[r.size()-1] == ']') {
        r = r.substr(0, with_args);
    }

    return r;

}

const char *TernException::what() const throw() {
    return _msg.c_str();
}

const char *SyscallException::what() const throw() {
    return _msg.c_str();
}

const char *FatalException::what() const throw() {
    return _msg.c_str();
}
