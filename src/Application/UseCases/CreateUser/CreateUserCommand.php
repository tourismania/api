<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\CreateUser;

readonly class CreateUserCommand
{
    public function __construct(
        public string $firstName,
        public string $lastName,
        public string $email,
    ){}
}
