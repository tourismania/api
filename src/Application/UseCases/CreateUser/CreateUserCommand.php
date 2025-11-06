<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\CreateUser;

class CreateUserCommand
{
    public function __invoke(): CreateUserResult
    {
        return new CreateUserResult();
    }

}
